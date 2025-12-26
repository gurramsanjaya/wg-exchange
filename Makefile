ARCH ?= amd64
OS ?= linux
APPVERSION ?= 1.0.0
DEBUG ?= false
BUILD_PATH ?= build
TLS_PATH ?= tls
ROOT_TLS_SUFFIX ?= rootCA
ROOT_TLS_OUTPUTS := ${TLS_PATH}/${ROOT_TLS_SUFFIX}.key ${TLS_PATH}/${ROOT_TLS_SUFFIX}.pem
X509_CA_FLAGS := -CA "${TLS_PATH}/${ROOT_TLS_SUFFIX}.pem" -CAkey "${TLS_PATH}/${ROOT_TLS_SUFFIX}.key"

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


binaries: server client
	sha256sum ${SERVER_PATH} ${CLIENT_PATH} > ${BUILD_PATH}/sha256sums
	sha512sum ${SERVER_PATH} ${CLIENT_PATH} > ${BUILD_PATH}/sha512sums
	tar --zstd -cvf wg-exchange-${OS}-${ARCH}.tar.zst -C ${BUILD_PATH} .

# 4 params:=file_suffix, subject, req_section, x509 params
define gen_cert
	openssl genpkey -algorithm ED25519 -out ${TLS_PATH}/$(1).key
	openssl req -new -key ${TLS_PATH}/$(1).key -out ${TLS_PATH}/$(1).csr -section common -subj $(2) -config openssl.cnf
	openssl x509 -req -in ${TLS_PATH}/$(1).csr -out ${TLS_PATH}/$(1).pem -extfile openssl.cnf -extensions $(3) $(4)
	rm -f ${TLS_PATH}/$(1).csr
endef

root-tls: 
	@$(call gen_cert,${ROOT_TLS_SUFFIX}, "/C=JP/O=Stardust Crusaders/CN=Root CA" ,ca_extensions, -days 30 -key ${TLS_PATH}/${ROOT_TLS_SUFFIX}.key)


# don't trigger root-tls here
client-tls: ${ROOT_TLS_OUTPUTS}
	@$(call gen_cert,client, "/C=JP/O=Diamond Is Unbreakable/CN=WG-Client", client_extensions, -days 1 ${X509_CA_FLAGS} )
	cat ${TLS_PATH}/${ROOT_TLS_SUFFIX}.pem > ${TLS_PATH}/client_bundle.pem
	cat ${TLS_PATH}/client.pem >> ${TLS_PATH}/client_bundle.pem
	rm -rf ${TLS_PATH}/server.pem

server-tls: ${ROOT_TLS_OUTPUTS}
	@$(call gen_cert,server, "/C=JP/O=Steel Ball Run/CN=WG-Server", server_extensions, -days 1 ${X509_CA_FLAGS} )
	cat ${TLS_PATH}/${ROOT_TLS_SUFFIX}.pem > ${TLS_PATH}/server_bundle.pem
	cat ${TLS_PATH}/server.pem >> ${TLS_PATH}/server_bundle.pem
	rm -f ${TLS_PATH}/server.pem

all-tls: root-tls client-tls server-tls

all: binaries all-tls

.PHONY: server client 