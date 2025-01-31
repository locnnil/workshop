import re
import argparse
import yaml
from pathlib import Path
from jinja2 import Template

HTML_TEMPLATE = """<!DOCTYPE html>
<html lang="en">
<body>
    <h1>Coverage Map</h1>
    <table>
        <thead>
            <tr>
                <th>Name</th>
                <th>Category</th>
                <th>Type</th>
                <th>Tutorial</th>
                <th>How-to</th>
                <th>Explanation</th>
                <th>Reference</th>
                <th>Specs</th>
            </tr>
        </thead>
        <tbody>
            {% for artefact in artefacts %}<tr>
                <td>{{ artefact.name }}</td>
                <td>{{ artefact.category }}</td>
                <td>{{ artefact.type }}</td>
                <td>{{ artefact.tutorial }}</td>
                <td>{{ artefact.how_to }}</td>
                <td>{{ artefact.explanation }}</td>
                <td>{{ artefact.reference }}</td>
                <td>{{ artefact.specs }}</td>
            </tr>{% endfor %}
        </tbody>
    </table>
</body>
</html>
"""


def extract_artefacts(file_path):
    pattern = re.compile(r"^\s*\.\.\s+@artefact\s+(.+?)\s*$")
    artefacts = []
    with open(file_path, "r", encoding="utf-8") as f:
        for line_number, line in enumerate(f, start=1):
            match = pattern.match(line)
            if match:
                artefacts.append((match.group(1), line_number))
    return artefacts


def build_artefact_table(base_dir, coverage_data):
    artefact_map = {}
    subdirs = ["how-to", "explanation", "tutorial", "reference"]

    for subdir in subdirs:
        path = Path(base_dir) / subdir
        if not path.exists():
            continue

        for file_path in path.rglob("*.rst"):
            artefact_entries = extract_artefacts(file_path)
            rel_path = file_path.relative_to(base_dir)

            for name, line_number in artefact_entries:
                if name not in artefact_map:
                    artefact_map[name] = {
                        "category": coverage_data.get(name, {}).get(
                            "category", "Unknown"
                        ),
                        "type": coverage_data.get(name, {}).get("type", "Unknown"),
                        "specs": "<br>".join(
                            coverage_data.get(name, {}).get("specs", [])
                        ),
                        **{key.replace("-", "_"): set() for key in subdirs},
                    }

                artefact_map[name][subdir.replace("-", "_")].add(
                    f'<a href="{base_dir / rel_path}#L{line_number}">[{file_path.name}]</a>'
                )

    for artefact in artefact_map.values():
        for key in ["tutorial", "how_to", "explanation", "reference"]:
            artefact[key] = "<br>".join(sorted(artefact[key]))

    return artefact_map


def generate_html(output_file, artefact_table):
    template = Template(HTML_TEMPLATE)
    artefacts = [
        {"name": name, **details}
        for name, details in sorted(
            artefact_table.items(),
            key=lambda item: (item[1].get("category", "") + item[0]).lower(),
        )
    ]
    with open(output_file, "w", encoding="utf-8") as f:
        f.write(template.render(artefacts=artefacts))


def main():
    parser = argparse.ArgumentParser(
        description="Generate HTML-based Markdown for artefacts in reST files."
    )
    parser.add_argument(
        "directory",
        nargs="?",
        default=".",
        help="Directory to search (default: .)",
    )
    parser.add_argument(
        "--coverage",
        default=".coverage.yaml",
        help="Path to coverage YAML file (default: .coverage.yaml)",
    )
    parser.add_argument(
        "--output",
        default="coverage.md",
        help="Output file (default: coverage.md)",
    )
    args = parser.parse_args()

    with open(args.coverage, "r", encoding="utf-8") as f:
        coverage_data = yaml.safe_load(f)

    artefact_table = build_artefact_table(args.directory, coverage_data)
    generate_html(args.output, artefact_table)
    print(f"HTML output written to {args.output}")


if __name__ == "__main__":
    main()
