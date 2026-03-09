# Query Service Makefile

CC = gcc
CFLAGS = -Wall -Wextra -O2 -std=c99 -D_POSIX_C_SOURCE=200809L
LDFLAGS = -lsqlite3
TARGET = query
SRC = query.c

.PHONY: all clean test install

all: $(TARGET)

$(TARGET): $(SRC)
	$(CC) $(CFLAGS) -o $(TARGET) $(SRC) $(LDFLAGS)

clean:
	rm -f $(TARGET)
	rm -f *.o
	rm -f *.log
	rm -f /tmp/query.pid

test: all
	@echo "Running Query Service tests..."
	@echo "1. Testing configuration validation..."
	./$(TARGET) --validate
	@echo "2. Testing help..."
	./$(TARGET) --help

install: all
	@echo "Installing Query Service..."
	@mkdir -p /usr/local/bin
	@cp $(TARGET) /usr/local/bin/
	@echo "Installed to /usr/local/bin/$(TARGET)"

run: all
	./$(TARGET) --config query.ini

run-daemon: all
	./$(TARGET) --config query.ini --daemon

# Development targets
dev: CFLAGS += -g -DDEBUG
dev: all
