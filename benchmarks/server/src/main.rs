use affinity::*;
use libc;
use mio::{self, Token};
use std::env;
use std::io::ErrorKind::WouldBlock;
use std::io::Write;
use std::thread;
use std::time::{Duration, Instant};

fn main() {
    set_thread_affinity([7]).unwrap();

    let mut rate = 10; // in Hz

    let args: Vec<String> = env::args().collect();
    if args.len() > 1 {
        rate = args[1].parse::<usize>().unwrap();
    }
    let mut srv = Server::new(rate);
    srv.run();
}

fn nanos() -> u64 {
    let mut ts = libc::timespec {
        tv_sec: 0,
        tv_nsec: 0,
    };
    if unsafe { libc::clock_gettime(libc::CLOCK_MONOTONIC, &mut ts) } == 0 {
        let seconds = ts.tv_sec as u64;
        let nanoseconds = ts.tv_nsec as u32;
        Duration::new(seconds, nanoseconds).as_nanos() as u64
    } else {
        panic!("clock_gettime failed");
    }
}

struct Conn {
    conn: mio::net::TcpStream,
    buffer: [u8; 1024],
    writable: bool,
    period: Duration,
    last_write: Option<Instant>,
    id: usize,
}

impl Conn {
    fn new(conn: mio::net::TcpStream, period: Duration, id: usize) -> Conn {
        Conn {
            conn,
            buffer: [0; 1024],
            writable: true,
            period,
            last_write: None,
            id,
        }
    }

    // returns false when the connection is closed
    fn write(&mut self) -> bool {
        if !self.writable {
            return true;
        }

        let now = Instant::now();
        let can_write = match self.last_write {
            Some(t) => now.duration_since(t) >= self.period,
            None => true,
        };
        if can_write {
            self.buffer[..8].copy_from_slice(&nanos().to_le_bytes());
            match self.conn.write_all(&self.buffer) {
                Ok(_) => self.last_write = Some(now),
                Err(e) => {
                    if e.kind() == WouldBlock {
                        self.writable = false;
                    } else if e.kind() == std::io::ErrorKind::UnexpectedEof
                        || e.kind() == std::io::ErrorKind::ConnectionReset
                    {
                        // connection is closed
                        return false;
                    } else {
                        panic!("{}", e)
                    }
                }
            }
        }
        return true;
    }
}

struct Server {
    poll: mio::Poll,
    events: mio::Events,
    ln: mio::net::TcpListener,
    period: Duration,
}

const SERVER: mio::Token = mio::Token(10000);

fn next(current: &mut Token) -> Token {
    let next = current.0;
    current.0 += 1;
    Token(next)
}

impl Server {
    fn new(rate: usize) -> Server {
        let poll = mio::Poll::new().unwrap();
        let events = mio::Events::with_capacity(1024);
        let addr = "127.0.0.1:8080".parse().unwrap();
        let mut ln = mio::net::TcpListener::bind(addr).unwrap();
        poll.registry()
            .register(&mut ln, SERVER, mio::Interest::READABLE)
            .unwrap();
        let period = std::time::Duration::from_secs(1) / (rate as u32);
        println!(
            "sending every {:?}(rate={}Hz) on each connection",
            period, rate
        );
        Server {
            poll,
            events,
            ln,
            period,
        }
    }

    fn run(&mut self) {
        let mut conns: Vec<Option<Conn>> = Vec::new();
        let mut unique_token = Token(0);
        loop {
            self.poll
                .poll(&mut self.events, Some(std::time::Duration::from_secs(0)))
                .unwrap();

            for event in self.events.iter() {
                if event.token() == SERVER {
                    loop {
                        let (mut conn, addr) = match self.ln.accept() {
                            Ok((conn, addr)) => (conn, addr),
                            Err(e) => {
                                if e.kind() == std::io::ErrorKind::WouldBlock {
                                    break;
                                } else {
                                    panic!("{}", e);
                                }
                            }
                        };
                        let token = next(&mut unique_token);
                        if token >= SERVER {
                            panic!("cannot create more than {} connections", SERVER.0)
                        }
                        println!("{}: connected to {}", token.0, addr);
                        self.poll
                            .registry()
                            .register(&mut conn, token, mio::Interest::WRITABLE)
                            .unwrap();
                        conn.set_nodelay(true).unwrap();
                        conns.push(Some(Conn::new(conn, self.period, token.0)));
                    }
                } else {
                    let index = usize::from(event.token());
                    match unsafe { conns.get_unchecked_mut(index) } {
                        Some(conn) => {
                            if event.is_write_closed() {
                                if conn.id != index {
                                    panic!("invalid id for conn");
                                }
                                conns[index] = None;
                                println!("connection {} closed", index);
                                continue;
                            }

                            if event.is_writable() {
                                conn.writable = true;
                            }
                        }
                        None => {}
                    };
                }
            }

            let mut done = true;
            conns.iter_mut().for_each(|option| match option {
                Some(conn) => {
                    if !conn.write() {
                        println!("connection {} closed", conn.id);
                        *option = None;
                    } else {
                        done = false;
                    }
                }
                None => {}
            });

            if done && unique_token.0 > 0 {
                println!("all connections are closed, resetting");
                conns.clear();
                unique_token = Token(0);
                thread::sleep(Duration::from_secs(1));

                if conns.len() != 0 || unique_token.0 != 0 {
                    panic!("invalid reset");
                }
                println!("--------------- can accept new connections ---------------");
            }
        }
    }
}
