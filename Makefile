.phony: all
all: run

.phony: build
build:
	rm -rf .output
	go run cmd/generate/main.go \
		-verbose \
    	-data     "data" \
    	-download ".download" \
    	-output   ".output"

.phony: run
run: build
	rsync -a .output/ echeclus.uberspace.de:/var/www/virtual/floppnet/parkrun.flopp.net/
	ssh echeclus.uberspace.de chmod -R o=u /var/www/virtual/floppnet/parkrun.flopp.net
