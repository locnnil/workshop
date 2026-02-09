"""Update redirected links in Workshop documentation sources.

Usage examples:
    python update_redirected_links.py --dry-run
    python update_redirected_links.py --max-redirects 5 --verify
    python update_redirected_links.py --redirect-codes 301,308 --docs-dir docs/

The script parses Sphinx linkcheck output, updates URLs that permanently
redirect, and writes a summary report and a rollback manifest.
"""

from __future__ import annotations

import argparse
import json
import os
import re
import sys
import tempfile
import time
import urllib.error
import urllib.parse
import urllib.request
from collections import defaultdict
from dataclasses import dataclass
from pathlib import Path
from typing import Iterable, List, Optional, Sequence, Tuple


ALLOWED_EXTENSIONS = {".rst", ".md", ".txt"}
DEFAULT_REDIRECT_CODES = {"301", "308"}
SUMMARY_FILE = "redirect_fixes_summary.md"
MANIFEST_FILE = "redirect_fixes_manifest.json"
USER_AGENT = "workshop-linkcheck-fixer/1.0"


@dataclass(frozen=True)
class RedirectEntry:
    file_path: Path
    line_no: int
    status_code: Optional[str]
    old_url: str
    new_url: str
    raw_line: str


@dataclass
class ChangeRecord:
    file_path: Path
    line_no: int
    old_url: str
    new_url: str
    old_line: str
    new_line: str


def log(message: str) -> None:
    print(message, file=sys.stderr)


def parse_args(argv: Optional[Sequence[str]] = None) -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Update redirected links in Sphinx documentation sources.",
    )
    parser.add_argument(
        "--redirect-codes",
        default=",".join(sorted(DEFAULT_REDIRECT_CODES)),
        help="Comma-separated HTTP status codes to process (default: 301,308).",
    )
    parser.add_argument(
        "--max-redirects",
        type=int,
        default=3,
        help="Maximum number of redirects to fix per run (default: 3).",
    )
    parser.add_argument(
        "--dry-run",
        action="store_true",
        help="Show planned changes without modifying files.",
    )
    parser.add_argument(
        "--verify",
        action=argparse.BooleanOptionalAction,
        default=True,
        help="Verify updated links are reachable (default: True).",
    )
    parser.add_argument(
        "--docs-dir",
        default="docs/",
        help="Path to the docs directory (default: docs/).",
    )
    parser.add_argument(
        "--temp-dir",
        default=None,
        help="Directory for summaries and manifests (default: create a unique temp dir).",
    )
    return parser.parse_args(argv)


def parse_redirect_codes(value: str) -> set[str]:
    codes = {
        code.strip()
        for code in value.split(",")
        if code.strip() and code.strip().isdigit() and len(code.strip()) == 3
    }
    return codes or set(DEFAULT_REDIRECT_CODES)


def find_linkcheck_output(docs_dir: Path) -> Optional[Path]:
    candidates = [
        docs_dir / "_build" / "output.txt",
        docs_dir / "_build" / "linkcheck" / "output.txt",
    ]
    for candidate in candidates:
        if candidate.exists():
            return candidate
    return None


def read_linkcheck_text(docs_dir: Path) -> str:
    output_path = find_linkcheck_output(docs_dir)
    if output_path:
        log(f"Using linkcheck output file: {output_path}")
        return output_path.read_text(encoding="utf-8", errors="replace")
    if not sys.stdin.isatty():
        log("Reading linkcheck output from stdin.")
        stdin_data = sys.stdin.read()
        if stdin_data.strip():
            return stdin_data
        raise FileNotFoundError(
            "linkcheck output not found; run 'make linkcheck' or pipe output to stdin"
        )
    raise FileNotFoundError(
        "linkcheck output not found; run 'make linkcheck' or pipe output to stdin"
    )


def normalize_path(path_str: str, docs_dir: Path) -> Path:
    path = Path(path_str)
    if not path.is_absolute():
        path = docs_dir / path
    return path.resolve()


def is_in_docs(path: Path, docs_dir: Path) -> bool:
    try:
        path.relative_to(docs_dir)
        return True
    except ValueError:
        return False


def extract_status_code(text: str) -> Optional[str]:
    match = re.search(r"\b(30\d)\b", text)
    if match:
        return match.group(1)
    return None


def extract_urls(text: str) -> Optional[Tuple[str, str]]:
    url_pattern = r"https?://[^\s)]+"
    match = re.search(
        rf"(?P<old>{url_pattern})\s+(?:to|->)\s+(?P<new>{url_pattern})",
        text,
    )
    if not match:
        return None
    return sanitize_url(match.group("old")), sanitize_url(match.group("new"))


def sanitize_url(url: str) -> str:
    return url.rstrip(".,;)")


def build_request(url: str, method: str) -> urllib.request.Request:
    return urllib.request.Request(
        url,
        method=method,
        headers={"User-Agent": USER_AGENT},
    )


def determine_status_code(url: str, timeout: float = 5.0) -> Optional[str]:
    class NoRedirect(urllib.request.HTTPRedirectHandler):
        def redirect_request(self, req, fp, code, msg, headers, newurl):
            return None

    opener = urllib.request.build_opener(NoRedirect)
    request = build_request(url, method="HEAD")
    try:
        with opener.open(request, timeout=timeout) as response:
            return str(response.status)
    except urllib.error.HTTPError as exc:
        if exc.code in {405, 403}:
            try:
                with opener.open(
                    build_request(url, method="GET"), timeout=timeout
                ) as response:
                    return str(response.status)
            except urllib.error.HTTPError as fallback_exc:
                if fallback_exc.code:
                    return str(fallback_exc.code)
        if exc.code:
            return str(exc.code)
    except Exception:
        return None
    return None


def parse_redirect_lines(
    lines: Iterable[str],
    docs_dir: Path,
    allowed_codes: set[str],
) -> Tuple[List[RedirectEntry], List[str]]:
    entries: List[RedirectEntry] = []
    warnings: List[str] = []
    for raw_line in lines:
        if "[redirected" not in raw_line:
            continue
        file_match = re.match(
            r"^(?P<file>.+?):\s*line\s*(?P<line>\d+):\s*\[redirected[^\]]*\]\s*(?P<rest>.+)$",
            raw_line,
        )
        if not file_match:
            file_match = re.match(
                r"^(?P<file>.+?):(?P<line>\d+):\s*\[redirected[^\]]*\]\s*(?P<rest>.+)$",
                raw_line,
            )
        if not file_match:
            warnings.append(f"Unrecognized redirect line: {raw_line.strip()}")
            continue

        file_path = normalize_path(file_match.group("file"), docs_dir)
        line_no = int(file_match.group("line"))
        rest = file_match.group("rest")

        urls = extract_urls(rest)
        if not urls:
            warnings.append(f"Missing URLs in redirect line: {raw_line.strip()}")
            continue
        old_url, new_url = urls

        status_code = extract_status_code(rest)
        if status_code is None:
            status_code = determine_status_code(old_url)
        if status_code and status_code not in allowed_codes:
            continue
        if status_code is None and allowed_codes:
            warnings.append(
                f"Skipping redirect with unknown status code: {raw_line.strip()}"
            )
            continue

        entries.append(
            RedirectEntry(
                file_path=file_path,
                line_no=line_no,
                status_code=status_code,
                old_url=old_url,
                new_url=new_url,
                raw_line=raw_line.strip(),
            )
        )
    return entries, warnings


def preserve_query_fragment(old_url: str, new_url: str) -> str:
    old_parts = urllib.parse.urlsplit(old_url)
    new_parts = urllib.parse.urlsplit(new_url)
    query = new_parts.query or old_parts.query
    fragment = new_parts.fragment or old_parts.fragment
    updated = new_parts._replace(query=query, fragment=fragment)
    return urllib.parse.urlunsplit(updated)


def same_domain(old_url: str, new_url: str) -> bool:
    old_netloc = urllib.parse.urlsplit(old_url).netloc.lower()
    new_netloc = urllib.parse.urlsplit(new_url).netloc.lower()
    return old_netloc == new_netloc


def verify_url(url: str, timeout: float = 5.0) -> Optional[str]:
    request = build_request(url, method="HEAD")
    try:
        with urllib.request.urlopen(request, timeout=timeout) as response:
            return str(response.status)
    except urllib.error.HTTPError as exc:
        if exc.code in {405, 403}:
            try:
                with urllib.request.urlopen(
                    build_request(url, method="GET"), timeout=timeout
                ) as response:
                    return str(response.status)
            except urllib.error.HTTPError as fallback_exc:
                return f"HTTP {fallback_exc.code}"
        return f"HTTP {exc.code}"
    except Exception as exc:
        return str(exc)


def apply_changes_to_file(
    file_path: Path,
    changes: List[RedirectEntry],
    dry_run: bool,
) -> Tuple[List[ChangeRecord], List[str]]:
    errors: List[str] = []
    applied_changes: List[ChangeRecord] = []
    if not file_path.exists():
        errors.append(f"File not found: {file_path}")
        return applied_changes, errors
    try:
        lines = file_path.read_text(encoding="utf-8", errors="replace").splitlines(
            keepends=True
        )
    except Exception as exc:
        errors.append(f"Failed to read {file_path}: {exc}")
        return applied_changes, errors

    for entry in changes:
        if entry.line_no < 1 or entry.line_no > len(lines):
            errors.append(f"Line {entry.line_no} out of range for {file_path}")
            continue
        target_line = lines[entry.line_no - 1]
        if entry.old_url in target_line:
            new_line = target_line.replace(entry.old_url, entry.new_url)
            if new_line == target_line:
                continue
            lines[entry.line_no - 1] = new_line
            applied_changes.append(
                ChangeRecord(
                    file_path=file_path,
                    line_no=entry.line_no,
                    old_url=entry.old_url,
                    new_url=entry.new_url,
                    old_line=target_line.rstrip("\n"),
                    new_line=new_line.rstrip("\n"),
                )
            )
            continue

        occurrences = [
            index for index, line in enumerate(lines, start=1) if entry.old_url in line
        ]
        if len(occurrences) == 1:
            index = occurrences[0]
            target_line = lines[index - 1]
            new_line = target_line.replace(entry.old_url, entry.new_url)
            lines[index - 1] = new_line
            applied_changes.append(
                ChangeRecord(
                    file_path=file_path,
                    line_no=index,
                    old_url=entry.old_url,
                    new_url=entry.new_url,
                    old_line=target_line.rstrip("\n"),
                    new_line=new_line.rstrip("\n"),
                )
            )
        else:
            if any(entry.new_url in line for line in lines):
                log(f"URL already updated in {file_path} for {entry.old_url}")
                continue
            errors.append(f"URL not found uniquely in {file_path} for {entry.old_url}")

    if not dry_run and applied_changes:
        temp_path = file_path.with_suffix(file_path.suffix + ".tmp")
        temp_path.write_text("".join(lines), encoding="utf-8")
        os.replace(temp_path, file_path)
    return applied_changes, errors


def write_summary(
    docs_dir: Path,
    output_dir: Path,
    changes: List[ChangeRecord],
    skipped: List[str],
    warnings: List[str],
    verification_failures: List[str],
    redirect_codes: set[str],
) -> None:
    summary_path = output_dir / SUMMARY_FILE
    files_modified = {str(change.file_path) for change in changes}

    lines = [
        "# Redirect fixes summary\n",
        f"Generated: {time.strftime('%Y-%m-%d %H:%M:%S')}\n",
        "\n",
        "## Overview\n",
        f"- Files modified: {len(files_modified)}\n",
        f"- Redirects fixed: {len(changes)}\n",
        f"- Redirect codes processed: {', '.join(sorted(redirect_codes))}\n",
        "\n",
    ]

    if changes:
        lines.extend(
            [
                "## Changes\n",
                "| File | Line | Old URL | New URL |\n",
                "| --- | ---: | --- | --- |\n",
            ]
        )
        for change in changes:
            relative_path = change.file_path.relative_to(docs_dir)
            lines.append(
                f"| {relative_path} | {change.line_no} | {change.old_url} | {change.new_url} |\n"
            )
        lines.append("\n")

    if skipped:
        lines.append("## Skipped redirects\n")
        for item in skipped:
            lines.append(f"- {item}\n")
        lines.append("\n")

    if verification_failures:
        lines.append("## Verification failures\n")
        for failure in verification_failures:
            lines.append(f"- {failure}\n")
        lines.append("\n")

    if warnings:
        lines.append("## Warnings\n")
        for warning in warnings:
            lines.append(f"- {warning}\n")
        lines.append("\n")

    summary_path.write_text("".join(lines), encoding="utf-8")


def write_manifest(
    docs_dir: Path,
    output_dir: Path,
    changes: List[ChangeRecord],
    redirect_codes: set[str],
    verification_failures: List[str],
) -> None:
    manifest_path = output_dir / MANIFEST_FILE
    payload = {
        "generated_at": time.strftime("%Y-%m-%d %H:%M:%S"),
        "redirect_codes": sorted(redirect_codes),
        "verification_failures": verification_failures,
        "changes": [
            {
                "file": str(change.file_path.relative_to(docs_dir)),
                "line": change.line_no,
                "old_url": change.old_url,
                "new_url": change.new_url,
                "old_line": change.old_line,
                "new_line": change.new_line,
            }
            for change in changes
        ],
    }
    manifest_path.write_text(json.dumps(payload, indent=2), encoding="utf-8")


def main() -> int:
    args = parse_args()
    docs_dir = Path(args.docs_dir).resolve()
    if args.temp_dir:
        output_dir = Path(args.temp_dir).resolve()
        temp_dir_source = "explicit"
    else:
        output_dir = Path(tempfile.mkdtemp(prefix="workshop-linkcheck-fixes-"))
        temp_dir_source = "auto"
    redirect_codes = parse_redirect_codes(args.redirect_codes)

    log("Starting redirect update run.")
    log(f"Docs directory: {docs_dir}")
    log(f"Temp directory ({temp_dir_source}): {output_dir}")
    log(f"Redirect codes: {', '.join(sorted(redirect_codes))}")
    log(f"Max redirects: {args.max_redirects}")
    log(f"Dry run: {args.dry_run}")
    log(f"Verify links: {args.verify}")

    if not docs_dir.exists():
        log(f"Docs directory not found: {docs_dir}")
        return 1

    if not output_dir.exists():
        try:
            output_dir.mkdir(parents=True, exist_ok=True)
        except OSError as exc:
            log(f"Failed to create temp directory {output_dir}: {exc}")
            return 1

    try:
        linkcheck_text = read_linkcheck_text(docs_dir)
    except Exception as exc:
        log(str(exc))
        return 1

    entries, warnings = parse_redirect_lines(
        linkcheck_text.splitlines(),
        docs_dir,
        redirect_codes,
    )
    log(f"Redirect entries parsed: {len(entries)}")
    if not entries:
        write_summary(
            docs_dir,
            output_dir,
            changes=[],
            skipped=[],
            warnings=warnings,
            verification_failures=[],
            redirect_codes=redirect_codes,
        )
        return 2

    filtered_entries: List[RedirectEntry] = []
    skipped: List[str] = []
    for entry in entries:
        if not is_in_docs(entry.file_path, docs_dir):
            skipped.append(f"Outside docs dir: {entry.raw_line}")
            continue
        if entry.file_path.suffix not in ALLOWED_EXTENSIONS:
            skipped.append(f"Unsupported file type: {entry.raw_line}")
            continue
        if not same_domain(entry.old_url, entry.new_url):
            skipped.append(f"Domain changed: {entry.raw_line}")
            continue
        updated_url = preserve_query_fragment(entry.old_url, entry.new_url)
        if updated_url == entry.old_url:
            skipped.append(f"Redirect target unchanged: {entry.raw_line}")
            continue
        filtered_entries.append(
            RedirectEntry(
                file_path=entry.file_path,
                line_no=entry.line_no,
                status_code=entry.status_code,
                old_url=entry.old_url,
                new_url=updated_url,
                raw_line=entry.raw_line,
            )
        )

    if not filtered_entries:
        write_summary(
            docs_dir,
            output_dir,
            changes=[],
            skipped=skipped,
            warnings=warnings,
            verification_failures=[],
            redirect_codes=redirect_codes,
        )
        return 2

    filtered_entries = filtered_entries[: max(args.max_redirects, 0)]
    log(f"Redirect entries after filtering: {len(filtered_entries)}")

    grouped_entries: dict[Path, List[RedirectEntry]] = defaultdict(list)
    for entry in filtered_entries:
        grouped_entries[entry.file_path].append(entry)

    all_changes: List[ChangeRecord] = []
    errors: List[str] = []
    for file_path, changes in grouped_entries.items():
        applied, file_errors = apply_changes_to_file(
            file_path=file_path,
            changes=changes,
            dry_run=args.dry_run,
        )
        all_changes.extend(applied)
        errors.extend(file_errors)

    if errors:
        for error in errors:
            log(error)
        write_summary(
            docs_dir,
            output_dir,
            changes=all_changes,
            skipped=skipped,
            warnings=warnings,
            verification_failures=[],
            redirect_codes=redirect_codes,
        )
        return 1

    if not all_changes:
        write_summary(
            docs_dir,
            output_dir,
            changes=[],
            skipped=skipped,
            warnings=warnings,
            verification_failures=[],
            redirect_codes=redirect_codes,
        )
        return 2
    log(f"Applied changes: {len(all_changes)}")

    verification_failures: List[str] = []
    if args.verify and not args.dry_run:
        for change in all_changes:
            status = verify_url(change.new_url)
            if (
                status
                and not status.startswith(("2", "3"))
                and "HTTP 401" not in status
                and "HTTP 403" not in status
            ):
                verification_failures.append(f"{change.new_url} ({status})")
        log(f"Verification failures: {len(verification_failures)}")

    write_manifest(
        docs_dir, output_dir, all_changes, redirect_codes, verification_failures
    )
    write_summary(
        docs_dir,
        output_dir,
        changes=all_changes,
        skipped=skipped,
        warnings=warnings,
        verification_failures=verification_failures,
        redirect_codes=redirect_codes,
    )

    if verification_failures:
        return 3
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
