# LFD401: Linux Application Development Training
## 5-Day Intensive Course

---

## Course Overview

### Objectives

This comprehensive 5-day training course "LFD401 - Developing Applications for Linux" equips you with the skills to develop robust applications for Linux systems. The course provides hands-on experience with the essential tools and methodologies for Linux application development.

**By the end of this course, you will:**
- Understand the unique features and techniques that make Linux an outstanding operating system
- Gain deep insights into tools and methods for programming applications in C
- Master system programming under Linux
- Learn debugging techniques and process management
- Develop comprehensive understanding of Linux-specific interfaces and system calls
- Successfully apply your acquired knowledge across various leading Linux distributions
- Possess the necessary knowledge and practical skills to effectively develop applications for Linux

### Target Audience

This course is designed for **experienced software developers** who want to deepen their Linux application development expertise.

### Prerequisites

For optimal participation in the course, we recommend the following background:
- **Proficiency in C programming**
- **Familiarity with Linux utilities and text editors**
- Basic understanding of operating system concepts

### Learning Methodology

The training offers a balanced mix of theory and practice in a first-class learning environment. You will benefit from direct exchange with our project-experienced trainers and other participants to maximize your learning success. The course includes:
- Hands-on lab exercises
- Real-world code examples
- Interactive discussions
- Practical problem-solving sessions

---

## Detailed Course Agenda

### Day 1: Development Tools and Build Systems

#### 1. Compilers
- **GCC (GNU Compiler Collection)**
  - Understanding the compilation process
  - Compiler toolchain components
- **Alternative Compilers**
  - Clang/LLVM
  - Intel C/C++ Compiler
  - Comparison of compiler features
- **Important GCC Options**
  - Optimization flags (-O0, -O1, -O2, -O3, -Os)
  - Warning flags (-Wall, -Wextra, -Werror)
  - Debugging symbols (-g)
  - Architecture-specific options
- **Preprocessor**
  - Macro definitions and expansion
  - Conditional compilation
  - Include file handling
- **Integrated Development Environments (IDE)**
  - Eclipse CDT
  - Visual Studio Code
  - CLion and other modern IDEs

#### 2. Libraries
- **Static Libraries**
  - Creating and using .a files
  - Archive management with ar
  - Linking static libraries
- **Shared Libraries**
  - Creating and using .so files
  - Position Independent Code (PIC)
  - Library versioning and sonames
- **Linking to Libraries**
  - Link-time library resolution
  - Library search paths
  - Runtime library dependencies
- **Dynamic Linking with Loaders**
  - Understanding ld.so and ld-linux.so
  - /etc/ld.so.conf configuration
  - Using ldconfig
  - Examining dependencies with ldd

#### 3. Make and Build Systems
- **Using make and Makefiles**
  - Basic Makefile syntax
  - Targets, prerequisites, and recipes
  - Variables and automatic variables
- **Building Large Projects**
  - Multi-directory builds
  - Recursive make
  - Dependency management
- **Complex Rules**
  - Pattern rules and wildcards
  - Implicit and explicit rules
  - Phony targets
- **Built-in Rules**
  - Default compilation rules
  - Customizing built-in rules

#### 4. Source Control
- **Source Code Control Overview**
  - Importance of version control
  - Centralized vs. distributed systems
- **Legacy Systems: RCS and CVS**
  - Historical context
  - Basic operations
- **Subversion (SVN)**
  - Repository structure
  - Branching and merging
- **Git**
  - Distributed version control concepts
  - Basic operations (clone, commit, push, pull)
  - Branching strategies
  - Collaboration workflows
  - Git best practices

---

### Day 2: Debugging and System Fundamentals

#### 5. Debugging and Core Dumps
- **GDB (GNU Debugger)**
  - Starting and controlling programs
  - Breakpoints and watchpoints
  - Examining variables and memory
  - Stack traces and backtraces
  - Debugging multi-threaded applications
- **What are Core Dump Files?**
  - Core dump generation mechanism
  - When and why core dumps occur
- **Generating Core Dumps**
  - Setting core dump limits with ulimit
  - Configuring core dump patterns
  - Using gcore for live processes
- **Examining Core Dumps**
  - Loading core files in gdb
  - Post-mortem debugging
  - Extracting information from crashed processes

#### 6. Debugging Tools
- **Electric Fence**
  - Memory debugging and protection
  - Detecting buffer overruns and underruns
- **Timing Measurements**
  - time command
  - clock_gettime() and timing APIs
  - Performance timing techniques
- **Performance Measurement and Profiling**
  - gprof for profiling
  - perf tools
  - Flame graphs and visualization
- **Valgrind**
  - Memory leak detection with memcheck
  - Cache and branch prediction profiling
  - Thread error detection with helgrind
  - Callgrind for call graph profiling

#### 7. System Calls
- **System Calls vs. Library Functions**
  - Understanding the distinction
  - When to use each approach
  - Performance considerations
- **Structure of System Calls**
  - User space to kernel space transition
  - System call numbers and tables
  - Parameter passing mechanisms
- **Return Values and Error Handling**
  - errno and error codes
  - Using perror() and strerror()
  - Handling EINTR and signal interruption

#### 8. Memory Management and Allocation
- **Memory Management Overview**
  - Virtual memory concepts
  - Address spaces and memory layout
  - Stack vs. heap
- **Dynamic Allocation**
  - malloc(), calloc(), realloc(), free()
  - Memory allocation strategies
  - Avoiding memory leaks
  - Double-free and use-after-free bugs
- **Tuning malloc()**
  - mallopt() options
  - Alternative allocators (jemalloc, tcmalloc)
  - Memory allocation performance
- **Page Locking**
  - mlock() and mlockall()
  - Use cases for page locking
  - Real-time application considerations

---

### Day 3: File Systems and I/O

#### 9. Files and File Systems in Linux
- **Files, Directories, and Devices**
  - Everything is a file philosophy
  - Special file types (block, character, socket, pipe)
  - Device files and /dev
- **The Virtual File System (VFS)**
  - VFS layer architecture
  - File system abstraction
  - Inode and dentry structures
- **The ext2/ext3 File System**
  - Structure and features
  - Block groups and inodes
- **Journaling File Systems**
  - Purpose and benefits of journaling
  - Journal modes (data, ordered, writeback)
- **The ext4 File System**
  - Improvements over ext3
  - Extents and performance enhancements
  - Delayed allocation

#### 10. File Input/Output
- **UNIX File I/O**
  - Low-level file operations
  - File descriptors
  - Buffered vs. unbuffered I/O
- **Opening and Closing Files**
  - open(), creat(), close()
  - Open flags (O_RDONLY, O_WRONLY, O_RDWR, O_CREAT, O_TRUNC, etc.)
  - File permissions and mode bits
- **Reading, Writing, and Seeking**
  - read() and write() system calls
  - lseek() for file positioning
  - Handling partial reads/writes
- **Positional and Vector I/O**
  - pread() and pwrite()
  - readv() and writev() (scatter-gather I/O)
  - Performance benefits
- **Standard I/O Libraries**
  - stdio vs. system calls
  - fopen(), fclose(), fread(), fwrite()
  - Buffering modes (fully, line, unbuffered)
- **Large File Support (LFS)**
  - 64-bit file offsets
  - _FILE_OFFSET_BITS=64
  - off_t vs. off64_t

#### 11. Advanced File Operations
- **Stat Functions**
  - stat(), lstat(), fstat()
  - File metadata and attributes
  - struct stat members
- **Directory Functions**
  - opendir(), readdir(), closedir()
  - Creating and removing directories
  - Traversing directory trees
- **inotify**
  - File system event monitoring
  - Setting up watches
  - Event types and handling
- **Memory Mapping**
  - mmap() and munmap()
  - File-backed vs. anonymous mappings
  - Shared memory via mmap()
  - Memory-mapped I/O advantages
- **File Locking: flock() and fcntl()**
  - Advisory vs. mandatory locking
  - Shared and exclusive locks
  - Record locking with fcntl()
- **Creating Temporary Files**
  - mkstemp(), mkdtemp()
  - tmpfile() and tmpnam()
  - Security considerations
- **Other System Calls**
  - dup() and dup2()
  - fcntl() for file descriptor manipulation
  - ioctl() for device-specific operations

---

### Day 4: Process Management and IPC

#### 12. Processes - Part I
- **What is a Process?**
  - Process concept and lifecycle
  - Process states (running, sleeping, stopped, zombie)
  - Process ID (PID) and parent process ID (PPID)
- **Process Limits**
  - Resource limits (RLIMIT_*)
  - getrlimit() and setrlimit()
- **Process Groups**
  - Process groups and sessions
  - Job control
  - Foreground and background processes
- **The /proc File System**
  - Process information in /proc/[pid]/
  - Reading process status and statistics
  - System information in /proc
- **Inter-Process Communication Methods**
  - Overview of IPC mechanisms
  - Choosing the right IPC method

#### 13. Processes - Part II
- **Using system() to Create a Process**
  - How system() works
  - Security implications
  - Return value handling
- **Creating Processes with fork()**
  - fork() semantics
  - Copy-on-write mechanism
  - Parent and child process relationship
  - Handling fork() return values
- **Using exec() to Create a Process**
  - exec() family of functions
  - execl(), execv(), execle(), execve(), etc.
  - Environment variables
  - Combining fork() and exec()
- **Using clone()**
  - Fine-grained process creation
  - Sharing resources between processes
  - Namespace support
- **Process Termination**
  - exit() and _exit()
  - Return codes and conventions
  - Cleanup at termination
- **Constructors and Destructors**
  - __attribute__((constructor))
  - __attribute__((destructor))
  - Initialization and cleanup functions
- **Wait States**
  - wait() and waitpid()
  - Reaping child processes
  - Avoiding zombie processes
- **Daemon Processes**
  - Characteristics of daemons
  - Creating daemon processes
  - Daemonizing best practices
  - syslog for logging

#### 14. Pipes and FIFOs
- **Pipes and Inter-Process Communication**
  - Anonymous pipes concept
  - Unidirectional communication
  - Parent-child communication
- **popen() and pclose()**
  - High-level pipe interface
  - Executing commands with pipes
  - Reading command output
- **pipe() System Call**
  - Creating anonymous pipes
  - File descriptor pairs
  - Bidirectional communication with two pipes
- **Named Pipes (FIFOs)**
  - Creating FIFOs with mkfifo()
  - Persistent pipes in the file system
  - Unrelated process communication
- **splice(), vmsplice(), and tee()**
  - Zero-copy data transfer
  - Efficient pipe operations
  - Performance optimization

#### 15. Asynchronous Input/Output
- **What is Asynchronous I/O?**
  - Difference from synchronous I/O
  - Non-blocking vs. asynchronous
  - Use cases and benefits
- **The POSIX Asynchronous I/O API**
  - aio_read() and aio_write()
  - aio_error() and aio_return()
  - aio_suspend() and aio_cancel()
  - lio_listio() for batch operations
- **Linux Implementation**
  - Kernel AIO (io_submit, io_getevents)
  - libaio library
  - Performance characteristics

#### 16. Signals - Part I
- **What are Signals?**
  - Signal concept and purpose
  - Asynchronous event notification
  - Signal delivery mechanism
- **Available Signals**
  - Standard signals (SIGINT, SIGTERM, SIGKILL, etc.)
  - Real-time signals
  - Signal number ranges
- **Sending Signals**
  - kill() system call
  - killpg() for process groups
  - raise() to send signal to self
- **Alarms, Pause, and Sleep**
  - alarm() for timer signals
  - pause() to wait for signals
  - sleep(), usleep(), nanosleep()
- **Setting Up Signal Handlers**
  - signal() function (legacy)
  - Handling specific signals
  - Default and ignore actions
- **Signal Sets**
  - sigset_t type
  - sigemptyset(), sigfillset()
  - sigaddset(), sigdelset()
  - sigismember()
- **sigaction()**
  - Modern signal handling
  - struct sigaction
  - SA_RESTART and other flags

#### 17. Signals - Part II
- **Reentrancy and Signal Handlers**
  - Async-signal-safe functions
  - Avoiding deadlocks in handlers
  - Signal handler restrictions
- **Longjmp and Non-local Returns**
  - setjmp() and longjmp()
  - siglongjmp() and sigsetjmp()
  - Stack unwinding considerations
- **siginfo and sigqueue()**
  - Extended signal information
  - Sending signals with data
  - SA_SIGINFO flag
- **Real-Time Signals**
  - SIGRTMIN to SIGRTMAX
  - Reliable signal delivery
  - Signal queuing
  - Priority ordering

---

### Day 5: Threading, Networking, and IPC

#### 18. POSIX Threads - Part I
- **Multi-threading on Linux**
  - Threading models
  - NPTL (Native POSIX Thread Library)
  - Benefits and challenges of threading
- **Programming Structure Basics**
  - Thread-safe programming
  - Shared vs. thread-local storage
  - Critical sections
- **Creating and Destroying Threads**
  - pthread_create()
  - pthread_exit()
  - pthread_join() and pthread_detach()
  - Thread attributes
- **Signals and Threads**
  - Signal delivery to threads
  - pthread_sigmask()
  - pthread_kill()
  - Signal handling in multi-threaded programs
- **Forking vs. Threading**
  - Use case comparison
  - Performance considerations
  - fork() in multi-threaded programs

#### 19. POSIX Threads - Part II
- **Deadlocks and Race Conditions**
  - Common concurrency bugs
  - Deadlock prevention strategies
  - Race condition detection
- **Mutex Operations**
  - pthread_mutex_init() and pthread_mutex_destroy()
  - pthread_mutex_lock() and pthread_mutex_unlock()
  - pthread_mutex_trylock()
  - Mutex attributes (recursive, error-checking)
- **Semaphores**
  - Counting semaphores
  - sem_init(), sem_wait(), sem_post()
  - Named vs. unnamed semaphores
- **Futexes (Fast Userspace Mutexes)**
  - Low-level synchronization primitive
  - Kernel-assisted locking
  - Performance characteristics
- **Condition Variable Operations**
  - pthread_cond_init() and pthread_cond_destroy()
  - pthread_cond_wait() and pthread_cond_timedwait()
  - pthread_cond_signal() and pthread_cond_broadcast()
  - Avoiding spurious wakeups

#### 20. Networks and Sockets
- **Network Layers**
  - OSI model vs. TCP/IP model
  - Layer responsibilities
  - Protocol encapsulation
- **What are Sockets?**
  - Socket abstraction
  - Socket as IPC endpoint
  - Socket domains (AF_UNIX, AF_INET, AF_INET6)
- **Stream Sockets**
  - SOCK_STREAM type
  - TCP characteristics
  - Connection-oriented communication
- **Datagram Sockets**
  - SOCK_DGRAM type
  - UDP characteristics
  - Connectionless communication
- **Raw Sockets**
  - SOCK_RAW type
  - Direct access to IP layer
  - Packet crafting and analysis
- **Byte Ordering**
  - Big-endian vs. little-endian
  - Network byte order
  - htons(), htonl(), ntohs(), ntohl()

#### 21. Sockets - Addresses and Hosts
- **Socket Address Structures**
  - struct sockaddr
  - struct sockaddr_in (IPv4)
  - struct sockaddr_in6 (IPv6)
  - struct sockaddr_un (UNIX domain)
- **Converting IP Addresses**
  - inet_pton() and inet_ntop()
  - Legacy functions: inet_addr(), inet_ntoa()
  - String to binary and binary to string
- **Host Information**
  - gethostbyname() and gethostbyaddr() (legacy)
  - getaddrinfo() and getnameinfo() (modern)
  - DNS resolution
  - /etc/hosts file

#### 22. Sockets - Ports and Protocols
- **Service Port Information**
  - Well-known ports (0-1023)
  - Registered ports (1024-49151)
  - getservbyname() and getservbyport()
  - /etc/services file
- **Protocol Information**
  - getprotobyname() and getprotobynumber()
  - /etc/protocols file
  - Common protocols (TCP, UDP, ICMP)

#### 23. Sockets - Clients
- **Client Basics and Sequence**
  - Typical client workflow
  - Connection establishment
- **socket() System Call**
  - Creating socket endpoints
  - Domain, type, and protocol parameters
  - File descriptor allocation
- **connect() System Call**
  - Establishing connections
  - Blocking behavior
  - Connection timeouts
- **close() and shutdown()**
  - Closing sockets
  - Half-close with shutdown()
  - SHUT_RD, SHUT_WR, SHUT_RDWR
- **UNIX Domain Client**
  - Local socket communication
  - Performance benefits
  - File system paths
- **Internet Client**
  - TCP/UDP client implementation
  - IPv4 and IPv6 support
  - Error handling

#### 24. Sockets - Servers
- **Server Basics and Sequence**
  - Typical server workflow
  - Listening for connections
- **bind() System Call**
  - Associating address with socket
  - Port binding
  - Wildcard addresses (INADDR_ANY)
- **listen() System Call**
  - Marking socket for incoming connections
  - Backlog parameter
  - Accept queue
- **accept() System Call**
  - Accepting incoming connections
  - Blocking behavior
  - Obtaining client address information
- **UNIX Domain Server**
  - Local socket server implementation
  - Socket file permissions
  - Cleanup considerations
- **Internet Server**
  - TCP/UDP server implementation
  - Handling multiple clients
  - Server design patterns

#### 25. Sockets - Input/Output Operations
- **write() and read()**
  - Generic I/O operations
  - Stream-oriented semantics
  - Partial reads and writes
- **send() and recv()**
  - Socket-specific operations
  - Flags (MSG_DONTWAIT, MSG_PEEK, MSG_WAITALL)
  - Return value handling
- **sendto() and recvfrom()**
  - Datagram socket operations
  - Specifying destination/source addresses
  - UDP communication
- **sendmsg() and recvmsg()**
  - Advanced message passing
  - Scatter-gather I/O
  - Ancillary data (control messages)
  - File descriptor passing
- **sendfile()**
  - Zero-copy data transfer
  - Efficient file sending
  - In-kernel data movement
- **socketpair()**
  - Creating connected socket pairs
  - Bidirectional IPC
  - UNIX domain sockets

#### 26. Sockets - Options
- **Getting and Setting Socket Options**
  - Socket-level vs. protocol-level options
  - Option categories
- **fcntl() for Sockets**
  - File descriptor flags
  - Non-blocking mode (O_NONBLOCK)
  - Asynchronous I/O signals
- **ioctl() for Sockets**
  - Device and socket control
  - Interface configuration
  - Network interface queries
- **getsockopt() and setsockopt()**
  - SO_REUSEADDR, SO_REUSEPORT
  - SO_RCVBUF, SO_SNDBUF (buffer sizes)
  - SO_KEEPALIVE
  - TCP_NODELAY (disable Nagle algorithm)
  - Timeout options

#### 27. Netlink Sockets
- **What are Netlink Sockets?**
  - Kernel-userspace communication
  - Netlink families and protocols
  - Use cases and advantages
- **Opening Netlink Sockets**
  - AF_NETLINK domain
  - Protocol selection
  - Binding to netlink addresses
- **Netlink Messages**
  - Message format and structure
  - Netlink headers
  - Routing tables and network configuration

#### 28. Sockets - Multiplexing and Concurrent Servers
- **Multiplexed and Asynchronous Socket I/O**
  - Need for I/O multiplexing
  - Handling multiple connections
  - Event-driven programming
- **select() System Call**
  - File descriptor sets
  - Timeout specification
  - Limitations (fd_set size)
- **poll() System Call**
  - pollfd structures
  - Event flags (POLLIN, POLLOUT, POLLERR)
  - No file descriptor limit
- **pselect() and ppoll()**
  - Signal mask parameter
  - Atomic signal handling
  - Time specification improvements
- **epoll**
  - Linux-specific scalable I/O event notification
  - epoll_create(), epoll_ctl(), epoll_wait()
  - Edge-triggered vs. level-triggered
  - Performance for large numbers of connections
- **Signal-driven and Asynchronous I/O**
  - SIGIO handling
  - F_SETOWN and F_SETSIG
  - Asynchronous notification
- **Concurrent Servers**
  - Multi-process servers (fork per connection)
  - Multi-threaded servers (thread per connection)
  - Thread pools and worker processes
  - Event-driven architectures

#### 29. Inter-Process Communication (IPC)
- **IPC Methods Overview**
  - Comparison of IPC mechanisms
  - Performance characteristics
  - Use case selection criteria
- **POSIX IPC**
  - Modern IPC standards
  - Portable interfaces
  - /dev/shm and /dev/mqueue
- **System V IPC**
  - Legacy IPC mechanisms
  - Key-based identification
  - ipcs and ipcrm utilities

#### 30. Shared Memory
- **What is Shared Memory?**
  - Fastest IPC mechanism
  - Direct memory access
  - Synchronization needs
- **POSIX Shared Memory**
  - shm_open() and shm_unlink()
  - ftruncate() to set size
  - mmap() to map into address space
  - Named shared memory objects
- **System V Shared Memory**
  - shmget() to create/obtain shared memory
  - shmat() to attach to address space
  - shmdt() to detach
  - shmctl() for control operations
  - IPC keys and identifiers

#### 31. Semaphores
- **What is a Semaphore?**
  - Synchronization primitive
  - Counting semaphores
  - Binary semaphores (mutex equivalent)
- **POSIX Semaphores**
  - Named semaphores (sem_open(), sem_close(), sem_unlink())
  - Unnamed semaphores (sem_init(), sem_destroy())
  - sem_wait(), sem_trywait(), sem_timedwait()
  - sem_post() to release
- **System V Semaphores**
  - Semaphore sets
  - semget() to create/obtain semaphore set
  - semop() for atomic operations
  - semctl() for control operations
  - Complex operations on multiple semaphores

#### 32. Message Queues
- **What are Message Queues?**
  - Message passing IPC
  - Priority-based message ordering
  - Asynchronous communication
- **POSIX Message Queues**
  - mq_open() and mq_close()
  - mq_send() and mq_receive()
  - mq_notify() for asynchronous notification
  - Message priorities and attributes
  - /dev/mqueue filesystem
- **System V Message Queues**
  - msgget() to create/obtain queue
  - msgsnd() to send messages
  - msgrcv() to receive messages
  - msgctl() for control operations
  - Message types and filtering

---

## Course Completion

Upon successful completion of this course, participants will have:
- Comprehensive understanding of Linux application development
- Hands-on experience with system-level programming in C
- Mastery of debugging and profiling tools
- Expertise in process management and inter-process communication
- Proficiency in network programming with sockets
- Skills in multi-threaded application development
- Practical knowledge applicable across major Linux distributions

---

**Course Duration:** 5 Days  
**Level:** Advanced  
**Language:** English  
**Format:** Instructor-led with hands-on labs
