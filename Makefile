.PHONY: test

test:
	go test -v -race -cover ./...

# 'integration' 태그가 있는 테스트만 unshare 환경에서 실행
test-integration:
	@echo "Running integration tests with unshare..."
	@unshare -r -m go test -v -tags=integration ./...

.PHONY: test test-integration