ARCH ?= amd64
OS ?= linux
VERSION ?= 1.0.0
DEBUG ?= false
BUILD_PATH ?= build

SERVER_FILE := wge-server
CLIENT_FILE := wge-client

# modify these subjectNames as needed
ROOT_SUBJ ?= "/C=JP/O=Stardust Crusaders/CN=Root CA" 
CLIENT_SUBJ ?= "/C=JP/O=Diamond Is Unbreakable/CN=WG-Client"
SERVER_SUBJ ?= "/C=JP/O=Steel Ball Run/CN=WG-Server"

TLS_PATH ?= tls
ROOT_TLS_SUFFIX ?= rootCA

ROOT_TLS_OUTPUTS := ${TLS_PATH}/${ROOT_TLS_SUFFIX}.key ${TLS_PATH}/${ROOT_TLS_SUFFIX}.pem
CLIENT_TLS_OUTPUTS := ${TLS_PATH}/client.key ${TLS_PATH}/client.pem
SERVER_TLS_OUTPUTS := ${TLS_PATH}/server.key ${TLS_PATH}/server.pem
X509_CA_FLAGS := -CA "${TLS_PATH}/${ROOT_TLS_SUFFIX}.pem" -CAkey "${TLS_PATH}/${ROOT_TLS_SUFFIX}.key"


APPVERSION_LDF := -X 'wg-exchange/cmd.AppVersion=${VERSION}'
COMMIT_HASH_LDF := -X 'wg-exchange/cmd.CommitHash=$(shell git rev-parse --short HEAD)'
BUILD_TIMESTAMP_LDF := -X 'wg-exchange/cmd.BuildTimestamp=$(shell date -Is -u)'

ifeq (${DEBUG},true)
	GCFLAGS = all=-N -l
else 
	LDFLAGS += -s
endif

LDFLAGS += ${APPVERSION_LDF} ${COMMIT_HASH_LDF} ${BUILD_TIMESTAMP_LDF} ${DBUS_LDF}


.PHONY: server client binaries clean-binaries
server:
	GOOS=${OS} GOARCH=${ARCH} go build -o ${BUILD_PATH}/${SERVER_FILE} -gcflags="${GCFLAGS}" -ldflags="${LDFLAGS}" cmd/wge-server/main.go

client:
	GOOS=${OS} GOARCH=${ARCH} go build -o ${BUILD_PATH}/${CLIENT_FILE} -gcflags="${GCFLAGS}" -ldflags="${LDFLAGS}" cmd/wge-client/main.go


binaries: server client
	cd ${BUILD_PATH} && \
	sha256sum ${SERVER_FILE} ${CLIENT_FILE} > sha256sums && \
	sha512sum ${SERVER_FILE} ${CLIENT_FILE} > sha512sums
	tar --zstd -cvf wg-exchange-${OS}-${ARCH}-${VERSION}.tar.zst -C ${BUILD_PATH} .

clean-binaries:
	rm -rf build wg-exhcange-${OS}-${ARCH}-${VERSION}.tar.xst


## tls stuff here

# mind the spaces in the args when calling this
# 4 args:= file_suffix, subject, req_section, x509 params
define gen_cert
	openssl genpkey -algorithm ED25519 -out ${TLS_PATH}/$(1).key
	openssl req -new -key ${TLS_PATH}/$(1).key -out ${TLS_PATH}/$(1).csr -section common -subj $(2) -config openssl.cnf
	openssl x509 -req -in ${TLS_PATH}/$(1).csr -out ${TLS_PATH}/$(1).pem -extfile openssl.cnf -extensions $(3) $(4)
	rm -f ${TLS_PATH}/$(1).csr
endef


${ROOT_TLS_OUTPUTS}:
	@$(call gen_cert,${ROOT_TLS_SUFFIX}, ${ROOT_SUBJ}, ca_extensions, -days 30 -key ${TLS_PATH}/${ROOT_TLS_SUFFIX}.key)

${CLIENT_TLS_OUTPUTS}: ${ROOT_TLS_OUTPUTS}
	@$(call gen_cert,client, ${CLIENT_SUBJ}, client_extensions, -days 1 ${X509_CA_FLAGS} )
	cat ${TLS_PATH}/${ROOT_TLS_SUFFIX}.pem >> ${TLS_PATH}/client.pem

${SERVER_TLS_OUTPUTS}: ${ROOT_TLS_OUTPUTS}
	@$(call gen_cert,server, ${SERVER_SUBJ}, server_extensions, -days 1 ${X509_CA_FLAGS} )
	cat ${TLS_PATH}/${ROOT_TLS_SUFFIX}.pem >> ${TLS_PATH}/server.pem

.PHONY: client-tls server-tls root-tls all-tls clean-tls
client-tls: ${CLIENT_TLS_OUTPUTS}

server-tls: ${SERVER_TLS_OUTPUTS}

root-tls: ${ROOT_TLS_OUTPUTS}


all-tls: client-tls server-tls

clean-tls:
	rm -f ${ROOT_TLS_OUTPUTS} ${SERVER_TLS_OUTPUTS} ${CLIENT_TLS_OUTPUTS}


.PHONY: clean all
all: binaries all-tls

clean: clean-binaries clean-tls