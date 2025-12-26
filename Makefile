ARCH ?= amd64
OS ?= linux
APPVERSION ?= 1.0.0
DEBUG ?= false
BUILD_PATH ?= build

SERVER_PATH := ${BUILD_PATH}/server-${OS}-${ARCH}
CLIENT_PATH := ${BUILD_PATH}/client-${OS}-${ARCH}

APPVERSION_LDF := -X 'wg-exchange/cmd.AppVersion=${APPVERSION}'
COMMIT_HASH_LDF := -X 'wg-exchange/cmd.CommitHash=$(shell git rev-parse --short HEAD)'
BUILD_TIMESTAMP_LDF := -X 'wg-exchange/cmd.BuildTimestamp=${shell date -Is -u}'

ifeq (${DEBUG},true)
	GCFLAGS = all=-N -l
else 
	LDFLAGS += -s
endif

LDFLAGS += ${APPVERSION_LDF} ${COMMIT_HASH_LDF} ${BUILD_TIMESTAMP_LDF} ${DBUS_LDF}

server:
	GOOS=${OS} GOARCH=${ARCH} go build -o ${SERVER_PATH} -gcflags="${GCFLAGS}" -ldflags="${LDFLAGS}" cmd/wge-server/main.go

client:
	GOOS=${OS} GOARCH=${ARCH} go build -o ${CLIENT_PATH} -gcflags="${GCFLAGS}" -ldflags="${LDFLAGS}" cmd/wge-client/main.go


all: server client
	sha256sum ${SERVER_PATH} ${CLIENT_PATH} > ${BUILD_PATH}/sha256sums
	sha512sum ${SERVER_PATH} ${CLIENT_PATH} > ${BUILD_PATH}/sha512sums
	tar --zstd -cvf wg-exchange-${OS}-${ARCH}.tar.zst -C ${BUILD_PATH} .

.PHONY: server client