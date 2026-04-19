.phony: all
all: run-remote

.phony: build
build:
	@echo "GENERATING HTML FILES..."
	@rm -rf .output
	@go run cmd/generate/main.go \
		-verbose \
    	-data     "data" \
    	-download ".download" \
		-output   ".output" \
		-config   "config.json"
	
.phony: run-local
run-local: build
	@echo "SERVING TO http://localhost:8080/"
	@python3 -m http.server --directory .output/ 8080


.phony: run-remote
run-remote: build
	@echo "DEPLOYING TO REMOTE SERVER..."
	@rsync -a .output/ echeclus.uberspace.de:/var/www/virtual/floppnet/parkruns.de/
	@ssh echeclus.uberspace.de chmod -R o=u /var/www/virtual/floppnet/parkruns.de

.phony: export
export:
	@rm -rf .output
	@go run cmd/generate/main.go \
		-verbose \
    	-data     "data" \
    	-download ".download" \
    	-output   ".output" \
		-config   "config.json" \
		-export-csv "parkrun-events.csv"