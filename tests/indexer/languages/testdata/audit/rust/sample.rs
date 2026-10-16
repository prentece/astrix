pub mod net {
    pub fn connect() {}
}

pub const MAX_CONN: usize = 16;
pub static VERSION: &str = "1.0";

pub type Result<T> = std::result::Result<T, String>;

pub enum Status {
    Active,
    Inactive,
}

pub trait Runner {
    fn run(&self) -> bool;
}

pub struct Server {
    pub port: u16,
    name: String,
}

impl Server {
    pub fn new(port: u16) -> Self {
        Self { port, name: String::new() }
    }

    pub fn start(&self) {
        net::connect();
    }
}

pub fn bootstrap() {
    let srv = Server::new(8080);
    srv.start();
}
