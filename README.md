# metrics

Client-server version of [premia](https://github.com/stevenwilkin/premia)


## Building

	go build ./cmd/metricsd
	go build ./cmd/metrics


## Running

	./metricsd

In another terminal:

	./metrics


## Systemd service

Copy the service unit file to the configuration directory:

	cp metrics.service /etc/systemd/system

Enable and start the service:

	systemctl enable metrics
	systemctl start metrics
