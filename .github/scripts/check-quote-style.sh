#!/usr/bin/env bash
# Enforce the quotation convention from docs/coding-style-guide.md.
# Accepts the list of staged files from pre-commit on argv.

set -u

fail=0
files=()
for f in "$@"; do
    case "$f" in
        cmd/*|internal/*|client/*) ;;
        *) continue ;;
    esac
    case "$f" in
        *_test.go) continue ;;
        internal/jsonutil/safejson/*) continue ;;
    esac
    case "$f" in
        *.go) files+=("$f") ;;
    esac
done

if [ "${#files[@]}" -eq 0 ]; then
    exit 0
fi

if grep -nP 'fmt\.(Errorf|Sprintf|Fprintf|Printf|Errorln|Println)\([^)]*\\"' "${files[@]}"; then
    echo 'quote-style: use backtick raw strings instead of \" escapes in fmt calls'
    fail=1
fi

if grep -nP "(fmt\.(Errorf|Sprintf|Fprintf|Printf|Errorln|Println)|errors\.New)\([^)]*'[A-Za-z_][A-Za-z0-9_/-]*'" "${files[@]}"; then
    echo "quote-style: use double quotes (or %q) instead of single quotes for literal tokens"
    fail=1
fi

if grep -nP 'errors\.New\("[^"]*\\"' "${files[@]}"; then
    echo 'quote-style: use backtick raw strings in errors.New instead of \" escapes'
    fail=1
fi

exit $fail
