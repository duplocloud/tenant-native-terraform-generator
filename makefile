BINARY=tenant-native-terraform-generator

build:
	go build -o ${BINARY}

run:
	go run main.go