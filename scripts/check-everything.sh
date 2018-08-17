RUN_DIR="$(cd "$(dirname "$0")" && pwd)"

"$RUN_DIR"/run_unit_tests.sh

echo "Running Linter"
cd "$RUN_DIR"/.. || exit 1
golangci-lint run
