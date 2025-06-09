# Base backend build step
FROM golang:1.24-bullseye AS go-base

ARG APP_BASE_DIR

# Support CGO and SSL
RUN apt update
RUN apt install gcc g++ make -y
RUN apt install git -y
RUN apt install vim -y
WORKDIR ${APP_BASE_DIR}
COPY ./ .

CMD ["sleep", "infinity"]