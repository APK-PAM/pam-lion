FROM node:14.16 as ui-build
ARG NPM_REGISTRY="https://registry.npmmirror.com"
ENV NPM_REGISTY=$NPM_REGISTRY

WORKDIR /opt/lion
RUN npm config set registry ${NPM_REGISTRY}
RUN yarn config set registry ${NPM_REGISTRY}

COPY ui  ui/
RUN ls . && cd ui/ && yarn install && yarn build && ls -al .

FROM golang:1.18-bullseye as stage-build
LABEL stage=stage-build
WORKDIR /opt/lion

ARG TARGETARCH
ARG GOPROXY=https://goproxy.cn
ENV CGO_ENABLED=0
ENV GO111MODULE=on
ENV GOOS=linux

COPY go.mod  .
COPY go.sum  .
RUN go mod download -x
COPY . .
ARG VERSION
ENV VERSION=$VERSION
RUN export GOFlAGS="-X 'main.Buildstamp=`date -u '+%Y-%m-%d %I:%M:%S%p'`'" \
	&& export GOFlAGS="${GOFlAGS} -X 'main.Githash=`git rev-parse HEAD`'" \
	&& export GOFlAGS="${GOFlAGS} -X 'main.Goversion=`go version`'" \
	&& export GOFlAGS="${GOFlAGS} -X 'main.Version=${VERSION}'" \
	&& go build -trimpath -x -ldflags "$GOFlAGS" -o lion . && ls -al .

FROM jumpserver/guacd:1.4.0
ARG TARGETARCH

USER root
WORKDIR /opt/lion

ARG DEPENDENCIES="                    \
        ca-certificates               \
        curl                          \
        locales                       \
        supervisor                    \
        telnet"

RUN sed -i 's@http://.*.debian.org@http://mirrors.ustc.edu.cn@g' /etc/apt/sources.list \
    && apt-get update \
    && apt-get install -y --no-install-recommends ${DEPENDENCIES} \
    && echo "zh_CN.UTF-8" | dpkg-reconfigure locales \
    && apt-get clean all \
    && rm -rf /var/lib/apt/lists/*

COPY --from=ui-build /opt/lion/ui/lion ui/lion/
COPY --from=stage-build /opt/lion/lion .
COPY --from=stage-build /opt/lion/config_example.yml .
COPY --from=stage-build /opt/lion/entrypoint.sh .
COPY --from=stage-build /opt/lion/supervisord.conf /etc/supervisor/conf.d/supervisord.conf

RUN chmod +x entrypoint.sh

ENV LANG=zh_CN.UTF-8

EXPOSE 8081
CMD ["./entrypoint.sh"]
