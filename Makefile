test:
	go test -v -count=1 ./...
test-deadlock:
	go test -v -count=1 -run ".*Deadlock.*" ./...
