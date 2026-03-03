#!/usr/bin/env bash
set -euo pipefail

REGION="${AWS_REGION:-us-east-1}"
TOPIC_NAME="accountlink-events"

echo "Creating SNS topic '${TOPIC_NAME}' in LocalStack (${REGION})..."

awslocal sns create-topic \
  --name "${TOPIC_NAME}" \
  --region "${REGION}" >/dev/null

ARN="$(awslocal sns list-topics --region "${REGION}" \
  | grep -o 'arn:aws:sns:[^"]*' | head -n 1)"

echo "LocalStack SNS topic ARN: ${ARN}"

# Soft: https://patorjk.com/software/taag/#p=display&f=Soft&t=READY%21%21%21&x=none&v=4&h=4&w=80&we=false
cat <<'EOF'
                                             ,---.,---.,---.
,------. ,------.  ,---.  ,------.,--.   ,--.|   ||   ||   |
|  .--. '|  .---' /  O  \ |  .-.  \\  `.'  / |  .'|  .'|  .'
|  '--'.'|  `--, |  .-.  ||  |  \  :'.    /  |  | |  | |  |
|  |\  \ |  `---.|  | |  ||  '--'  /  |  |   `--' `--' `--'
`--' '--'`------'`--' `--'`-------'   `--'   .--. .--. .--.
                                             '--' '--' '--'
EOF