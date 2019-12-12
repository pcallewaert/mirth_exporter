build:
	go build

docker:
	docker build -t pieterc/mirth_exporter .

push:
	docker push pieterc/mirth_exporter