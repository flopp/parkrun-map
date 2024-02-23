.phony: all
all:
	echo ""

.repo/.git/config:
	git clone https://github.com/flopp/parkrun-map.git .repo

.phony: sync
sync: .repo/.git/config
	(cd .repo && git pull --quiet)
	ssh echeclus.uberspace.de mkdir -p packages/parkrun-map
	scp scripts/cronjob.sh echeclus.uberspace.de:packages/parkrun-map
	ssh echeclus.uberspace.de chmod +x packages/parkrun-map/cronjob.sh
	rsync -a .repo/ echeclus.uberspace.de:packages/parkrun-map/repo

.phony: run
run: sync
	ssh echeclus.uberspace.de packages/parkrun-map/cronjob.sh

.phony: run-local
run-local:
	rm -rf .output
	go run cmd/generate/main.go \
    	-data     "data" \
    	-download ".download" \
    	-output   ".output"

.phony: upload-local
upload-local: run-local
	rsync -a .output/ echeclus.uberspace.de:/var/www/virtual/floppnet/parkrun.flopp.net/
	ssh echeclus.uberspace.de chmod -R o=u /var/www/virtual/floppnet/parkrun.flopp.net

