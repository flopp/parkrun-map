.phony: all
all: run

.phony: build
build:
	rm -rf .output
	go run cmd/generate/main.go \
		-verbose \
    	-data     "data" \
    	-download ".download" \
		-output   ".output" \
		-config   "config.json"

.phony: run
run: build
	rsync -a .output/ echeclus.uberspace.de:/var/www/virtual/floppnet/parkruns.de/
	ssh echeclus.uberspace.de chmod -R o=u /var/www/virtual/floppnet/parkruns.de

.phony: export
export:
	rm -rf .output
	go run cmd/generate/main.go \
		-verbose \
    	-data     "data" \
    	-download ".download" \
    	-output   ".output" \
		-config   "config.json" \
		-export-csv "parkrun-events.csv"