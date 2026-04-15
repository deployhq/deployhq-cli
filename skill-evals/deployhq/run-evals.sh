#!/usr/bin/env bash
#
# DeployHQ CLI Skill Eval Runner
#
# Tests whether an LLM correctly translates natural language into dhq commands.
# Uses the Claude API (requires ANTHROPIC_API_KEY) or any OpenAI-compatible API.
#
# Usage:
#   ./run-evals.sh                    # Run all evals
#   ./run-evals.sh --category deployments  # Run one category
#   ./run-evals.sh --id deploy-basic  # Run single eval
#   ./run-evals.sh --dry-run          # Show prompts without calling API
#   ./run-evals.sh --verbose          # Show full LLM responses
#
# Environment:
#   ANTHROPIC_API_KEY   - Required (unless --dry-run)
#   EVAL_MODEL          - Model to test (default: claude-sonnet-4-20250514)
#   SKILL_FILE          - Path to SKILL.md (default: auto-detected)

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
EVALS_FILE="$SCRIPT_DIR/evals.json"
SKILL_FILE="${SKILL_FILE:-$SCRIPT_DIR/../../skills/deployhq/SKILL.md}"
MODEL="${EVAL_MODEL:-claude-sonnet-4-20250514}"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
CYAN='\033[0;36m'
BOLD='\033[1m'
RESET='\033[0m'

# Parse args
CATEGORY=""
EVAL_ID=""
DRY_RUN=false
VERBOSE=false

while [[ $# -gt 0 ]]; do
  case $1 in
    --category) CATEGORY="$2"; shift 2 ;;
    --id) EVAL_ID="$2"; shift 2 ;;
    --dry-run) DRY_RUN=true; shift ;;
    --verbose) VERBOSE=true; shift ;;
    --model) MODEL="$2"; shift 2 ;;
    --help|-h)
      echo "Usage: $0 [--category <cat>] [--id <id>] [--dry-run] [--verbose] [--model <model>]"
      exit 0
      ;;
    *) echo "Unknown arg: $1"; exit 1 ;;
  esac
done

if ! command -v jq &>/dev/null; then
  echo "Error: jq is required. Install with: brew install jq"
  exit 1
fi

if [[ "$DRY_RUN" == false ]] && [[ -z "${ANTHROPIC_API_KEY:-}" ]]; then
  echo "Error: ANTHROPIC_API_KEY is required (or use --dry-run)"
  exit 1
fi

if [[ ! -f "$EVALS_FILE" ]]; then
  echo "Error: evals.json not found at $EVALS_FILE"
  exit 1
fi

# Load skill context
SKILL_CONTEXT=""
if [[ -f "$SKILL_FILE" ]]; then
  SKILL_CONTEXT=$(cat "$SKILL_FILE")
fi

# Load references for richer context
REFS_DIR="$SCRIPT_DIR/../../skills/deployhq/references"
REF_CONTEXT=""
if [[ -d "$REFS_DIR" ]]; then
  for ref in "$REFS_DIR"/*.md; do
    [[ -f "$ref" ]] && REF_CONTEXT+="$(cat "$ref")"$'\n\n'
  done
fi

# Build eval list
if [[ -n "$EVAL_ID" ]]; then
  EVAL_FILTER=".evals[] | select(.id == \"$EVAL_ID\")"
elif [[ -n "$CATEGORY" ]]; then
  EVAL_FILTER=".evals[] | select(.category == \"$CATEGORY\")"
else
  EVAL_FILTER=".evals[]"
fi

TOTAL=$(jq "[$EVAL_FILTER] | length" "$EVALS_FILE")
if [[ "$TOTAL" -eq 0 ]]; then
  echo "No evals matched filter."
  exit 1
fi

echo -e "${BOLD}DeployHQ CLI Skill Evals${RESET}"
echo -e "Model: ${CYAN}$MODEL${RESET}"
echo -e "Evals: ${CYAN}$TOTAL${RESET}"
echo -e "Mode:  ${CYAN}$(if $DRY_RUN; then echo "dry-run"; else echo "live"; fi)${RESET}"
echo ""

PASSED=0
FAILED=0
ERRORS=0

call_claude() {
  local prompt="$1"
  local system_msg="You are an AI agent that translates natural language into DeployHQ CLI (dhq) commands. Given a user request, output ONLY the exact CLI command(s) to run. No explanations, no markdown, no code blocks — just the raw command(s), one per line.

Context:
$SKILL_CONTEXT

$REF_CONTEXT"

  local payload
  payload=$(jq -n \
    --arg model "$MODEL" \
    --arg system "$system_msg" \
    --arg prompt "$prompt" \
    '{
      model: $model,
      max_tokens: 256,
      system: $system,
      messages: [{role: "user", content: $prompt}]
    }')

  local response
  response=$(curl -s -w "\n%{http_code}" \
    https://api.anthropic.com/v1/messages \
    -H "Content-Type: application/json" \
    -H "x-api-key: $ANTHROPIC_API_KEY" \
    -H "anthropic-version: 2023-06-01" \
    -d "$payload")

  local http_code
  http_code=$(echo "$response" | tail -1)
  local body
  body=$(echo "$response" | sed '$d')

  if [[ "$http_code" != "200" ]]; then
    echo "API_ERROR: HTTP $http_code"
    return 1
  fi

  echo "$body" | jq -r '.content[0].text // "EMPTY"'
}

check_eval() {
  local eval_json="$1"
  local response="$2"

  local expected_cmd
  expected_cmd=$(echo "$eval_json" | jq -r '.expected.command')

  # Check command is present
  if ! echo "$response" | grep -qF "$expected_cmd"; then
    echo "MISSING_COMMAND:$expected_cmd"
    return 1
  fi

  # Check required flags
  local flags
  flags=$(echo "$eval_json" | jq -r '.expected.flags[]? // empty')
  for flag in $flags; do
    if ! echo "$response" | grep -qF "$flag"; then
      echo "MISSING_FLAG:$flag"
      return 1
    fi
  done

  # Check args
  local args
  args=$(echo "$eval_json" | jq -r '.expected.args[]? // empty')
  for arg in $args; do
    if ! echo "$response" | grep -qF "$arg"; then
      echo "MISSING_ARG:$arg"
      return 1
    fi
  done

  # Check must_not_contain
  local forbidden
  forbidden=$(echo "$eval_json" | jq -r '.must_not_contain[]? // empty')
  for banned in $forbidden; do
    if echo "$response" | grep -qF "$banned"; then
      echo "FORBIDDEN:$banned"
      return 1
    fi
  done

  echo "PASS"
  return 0
}

# Run evals
INDEX=0
jq -c "[$EVAL_FILTER][]" "$EVALS_FILE" | while IFS= read -r eval_entry; do
  INDEX=$((INDEX + 1))
  id=$(echo "$eval_entry" | jq -r '.id')
  category=$(echo "$eval_entry" | jq -r '.category')
  prompt=$(echo "$eval_entry" | jq -r '.prompt')
  expected_cmd=$(echo "$eval_entry" | jq -r '.expected.command')

  printf "[%d/%d] %-30s " "$INDEX" "$TOTAL" "$id"

  if $DRY_RUN; then
    echo -e "${YELLOW}SKIP${RESET} (dry-run)"
    echo "  Prompt:   $prompt"
    echo "  Expected: $expected_cmd"
    continue
  fi

  # Call LLM
  response=$(call_claude "$prompt" 2>&1) || true

  if [[ "$response" == API_ERROR* ]]; then
    echo -e "${RED}ERROR${RESET} $response"
    ERRORS=$((ERRORS + 1))
    continue
  fi

  if $VERBOSE; then
    echo ""
    echo "  Response: $response"
  fi

  # Check result
  result=$(check_eval "$eval_entry" "$response")

  if [[ "$result" == "PASS" ]]; then
    echo -e "${GREEN}PASS${RESET}"
    PASSED=$((PASSED + 1))
  else
    echo -e "${RED}FAIL${RESET} ($result)"
    if ! $VERBOSE; then
      echo "  Expected: $expected_cmd"
      echo "  Got:      $(echo "$response" | head -1)"
    fi
    FAILED=$((FAILED + 1))
  fi

  # Rate limit courtesy
  sleep 0.5
done

echo ""
echo -e "${BOLD}Results${RESET}"
echo -e "  ${GREEN}Passed: $PASSED${RESET}"
echo -e "  ${RED}Failed: $FAILED${RESET}"
if [[ $ERRORS -gt 0 ]]; then
  echo -e "  ${YELLOW}Errors: $ERRORS${RESET}"
fi
echo -e "  Total:  $TOTAL"

if [[ $FAILED -gt 0 ]] || [[ $ERRORS -gt 0 ]]; then
  exit 1
fi
