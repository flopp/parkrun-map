.phony: all
all:
	echo "run - build & sync"

.phony: run
run:
	rm -rf .output
	go run cmd/generate/main.go \
		-verbose \
    	-data     "data" \
    	-download ".download" \
    	-output   ".output"

.phony: upload
upload: run
	rsync -a .output/ echeclus.uberspace.de:/var/www/virtual/floppnet/parkrun.flopp.net/
	ssh echeclus.uberspace.de chmod -R o=u /var/www/virtual/floppnet/parkrun.flopp.net
