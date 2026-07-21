ROOT_DIR := $(abspath ../..)
PROTOBUF_MANIFEST ?= $(ROOT_DIR)/config/release/protobuf-toolchain.v1.json
PROTOBUF_TOOL_BIN := $(ROOT_DIR)/.tmp/protobuf-tools/bin

.PHONY: proto-gen proto-verify

# Generation is verification-only: all outputs are written to a fresh temporary tree.
proto-gen proto-verify:
	bash "$(ROOT_DIR)/scripts/bootstrap-protobuf-tools.sh" --manifest "$(PROTOBUF_MANIFEST)"
	python3 "$(ROOT_DIR)/scripts/verify_protobuf_contract.py" \
		--manifest "$(PROTOBUF_MANIFEST)" \
		--tool-bin "$(PROTOBUF_TOOL_BIN)" \
		--regenerate
