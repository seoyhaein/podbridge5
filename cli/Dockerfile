# Use the official Alpine image as a base
FROM docker.io/library/ubuntu:latest

# Add a label to the image
LABEL maintainer="seoyhaein@gmail.com"

# Set environment variables
ENV HELLO "Hello, World!"

# Run a command to print the "Hello, World!" message
#CMD echo $HELLO

# 컨테이너가 종료되지 않도록 Bash에서 무한 루프 실행
CMD ["sh", "-c", "while :; do sleep 2073600; done"]
