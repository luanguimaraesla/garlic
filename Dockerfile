# garlic/Dockerfile
FROM golang:1.26 AS garlic-source

WORKDIR /garlic
COPY . .
