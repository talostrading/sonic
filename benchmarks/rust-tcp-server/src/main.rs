use std::net::{TcpListener, TcpStream};
use std::thread;
use std::io::{Read, Write};
use std::time::{Instant};

struct OnlineStats {
    sum: i64,
    sum_sq: i64,
    pub count: i64,
    min: i64,
    max: i64,
}

impl OnlineStats {
    fn new() -> OnlineStats {
        OnlineStats {
            sum: 0,
            sum_sq: 0,
            count: 0,
            min: std::i64::MAX,
            max: std::i64::MIN,
        }
    }

    fn reset(&mut self) {
        self.sum = 0;
        self.sum_sq = 0;
        self.count = 0;
        self.min = std::i64::MAX;
        self.max = std::i64::MIN;
    }

    fn record(&mut self, x: i64) {
        self.count += 1;
        self.sum += x;
        self.sum_sq += x * x;
        self.min = std::cmp::min(self.min, x);
        self.max = std::cmp::max(self.max, x);  
    }

    fn print(&self) {
        let avg = self.sum / self.count;
        let stddev = ((self.sum_sq / self.count - avg * avg) as f64).sqrt();
        println!("min/avg/max/stddev = {}/{}/{}/{}us", self.min, avg, self.max, stddev as i64);
    }
}

fn run(mut stream: TcpStream) {
    let mut stats = OnlineStats::new();
    loop {
        let start_time = Instant::now();
        let mut b: [u8; 128] = [0; 128];
        let _ = stream.write_all(&mut b);
        let n = stream.read(&mut b).unwrap();
        if n != 128 {
            panic!("did not read 128 bytes");
        }
        let end_time = Instant::now();
        let dur = end_time.duration_since(start_time).as_micros() as i64;       
        stats.record(dur);
        if stats.count > 50000 {
            stats.print();
            stats.reset();
        }
    }
}

fn main() {
    let ln = TcpListener::bind("localhost:1234").unwrap();
    let mut i = 0;
    for stream in ln.incoming() {
        i += 1;
        println!("accepted {}", i);
        let stream = stream.expect("failed to accept connection");
        thread::spawn(move || {
            run(stream);
        });
    }
}
