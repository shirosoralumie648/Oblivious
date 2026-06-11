.PHONY: proto-gen

proto-gen:
	protoc -I../../api/proto \
	       --go_out=internal/grpc/agentv1 --go_opt=paths=source_relative \
	       --go-grpc_out=internal/grpc/agentv1 --go-grpc_opt=paths=source_relative \
	       ../../api/proto/agent.proto
	protoc -I../../api/proto \
	       --go_out=internal/grpc/billingv1 --go_opt=paths=source_relative \
	       --go-grpc_out=internal/grpc/billingv1 --go-grpc_opt=paths=source_relative \
	       ../../api/proto/billing.proto
	protoc -I../../api/proto \
	       --go_out=internal/grpc/ragv1 --go_opt=paths=source_relative \
	       --go-grpc_out=internal/grpc/ragv1 --go-grpc_opt=paths=source_relative \
	       ../../api/proto/rag.proto
	protoc -I../../api/proto \
	       --go_out=internal/grpc/relayv1 --go_opt=paths=source_relative \
	       --go-grpc_out=internal/grpc/relayv1 --go-grpc_opt=paths=source_relative \
	       ../../api/proto/relay.proto
	protoc -I../../api/proto \
	       --go_out=internal/grpc/taskv1 --go_opt=paths=source_relative \
	       --go-grpc_out=internal/grpc/taskv1 --go-grpc_opt=paths=source_relative \
	       ../../api/proto/task.proto
	protoc -I../../api/proto \
	       --go_out=internal/grpc/workflowv1 --go_opt=paths=source_relative \
	       --go-grpc_out=internal/grpc/workflowv1 --go-grpc_opt=paths=source_relative \
	       ../../api/proto/workflow.proto
