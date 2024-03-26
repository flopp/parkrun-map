.phony: all
all:
	echo ""

.repo/.git/config:
	git clone https://github.com/flopp/parkrun-map.git .repo

.phony: .bin/generate-linux
.bin/generate-linux:
	mkdir -p .bin
	GOOS=linux GOARCH=amd64 go build -o .bin/generate-linux cmd/generate/main.go
	
.phony: sync
sync: .repo/.git/config .bin/generate-linux
	(cd .repo && git pull --quiet)
	ssh echeclus.uberspace.de mkdir -p packages/parkrun-map
	scp scripts/cronjob.sh .bin/generate-linux echeclus.uberspace.de:packages/parkrun-map
	ssh echeclus.uberspace.de chmod +x packages/parkrun-map/cronjob.sh packages/parkrun-map/generate-linux
	rsync -a .repo/ echeclus.uberspace.de:packages/parkrun-map/repo

.phony: run
run: sync
	ssh echeclus.uberspace.de packages/parkrun-map/cronjob.sh

.phony: run-local
run-local:
	rm -rf .output
	go run cmd/generate/main.go \
		-verbose \
    	-data     "data" \
    	-download ".download" \
    	-output   ".output"

.phony: upload-local
upload-local: run-local
	rsync -a .output/ echeclus.uberspace.de:/var/www/virtual/floppnet/parkrun.flopp.net/
	ssh echeclus.uberspace.de chmod -R o=u /var/www/virtual/floppnet/parkrun.flopp.net

.phony: run-local-world
run-local-world:
	rm -rf .output-world
	go run cmd/generate-world/main.go \
    	-data     "data" \
    	-download ".download-world" \
    	-output   ".output-world"

.phony: upload-local-world
upload-local-world: run-local-world
	rsync -a .output-world/ echeclus.uberspace.de:/var/www/virtual/floppnet/2oc.de/
	ssh echeclus.uberspace.de chmod -R o=u /var/www/virtual/floppnet/2oc.de