# garlic/Dockerfile
FROM golang:1.26.6 AS garlic-source

WORKDIR /garlic
COPY . .
