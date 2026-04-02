#!/bin/sh
set -eu

if [ ! -f "AGENTS.md" ]; then
  echo "AGENTS policy check failed: AGENTS.md not found at repository root."
  exit 1
fi

CHANGED_FILES="$(git diff --cached --name-only)"

if [ -z "$CHANGED_FILES" ]; then
  exit 0
fi

requires_test_update=0
has_test_update=0
requires_readme_update=0
has_readme_update=0

for file in $CHANGED_FILES; do
  case "$file" in
    internal/*.go|internal/*/*.go|internal/*/*/*.go)
      case "$file" in
        *_test.go) has_test_update=1 ;;
        *) requires_test_update=1 ;;
      esac
      ;;
  esac

  case "$file" in
    internal/config/config.go) requires_readme_update=1 ;;
    README.md) has_readme_update=1 ;;
  esac
done

if [ "$requires_test_update" -eq 1 ] && [ "$has_test_update" -eq 0 ] && [ "${ALLOW_NO_TEST_CHANGES:-0}" != "1" ]; then
  echo "AGENTS policy check failed:"
  echo "- Go source changed under internal/, but no *_test.go file is staged."
  echo "- Add/update tests or set ALLOW_NO_TEST_CHANGES=1 for this commit."
  exit 1
fi

if [ "$requires_readme_update" -eq 1 ] && [ "$has_readme_update" -eq 0 ] && [ "${ALLOW_NO_README_UPDATE:-0}" != "1" ]; then
  echo "AGENTS policy check failed:"
  echo "- internal/config/config.go changed, but README.md is not staged."
  echo "- Update docs or set ALLOW_NO_README_UPDATE=1 for this commit."
  exit 1
fi

exit 0
