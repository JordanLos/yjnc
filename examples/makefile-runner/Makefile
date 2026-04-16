# YJNC minimal runner — Makefile implementation
#
# Conforms to YJNC 1.0 for sequential and parallel execution.
# Does not implement: retry, checkpoint, sampling, or work-graph.json state persistence.
# For full conformance, use a runner that manages work-graph.json state.
#
# Usage:
#   make          run the full graph sequentially
#   make -j       run independent units concurrently
#   make verify   run from a specific target

.PHONY: all fetch analyze implement verify

all: verify

verify: implement
	@echo "==> verify"
	./verify/contract

implement: analyze
	@echo "==> implement"
	claude --agent implement/agent.md
	./implement/contract

analyze: fetch
	@echo "==> analyze"
	claude --agent analyze/agent.md
	./analyze/contract

fetch:
	@echo "==> fetch"
	claude --agent fetch/agent.md
	./fetch/contract
