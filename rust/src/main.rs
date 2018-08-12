extern crate serde;
extern crate serde_json;
#[macro_use]
extern crate serde_derive;
#[macro_use]
extern crate diesel;
extern crate actix;
extern crate actix_web;
extern crate env_logger;
extern crate futures;
extern crate r2d2;
extern crate uuid;
use actix::prelude::*;
use actix_web::{
	http, middleware, server, App, AsyncResponder, FutureResponse, HttpResponse, Path, State,
};

use diesel::prelude::*;
use diesel::r2d2::ConnectionManager;
use futures::Future;

mod db;
use db::{CreateUser, DbExecutor};
mod models;
mod schema;
struct AppState {
	db: Addr<DbExecutor>,
}

fn index((name, state): (Path<String>, State<AppState>)) -> FutureResponse<HttpResponse> {
	println!("{:?}", name);
	state
		.db
		.send(CreateUser {
			name: name.into_inner(),
		})
		.from_err()
		.and_then(|res| match res {
			Ok(user) => Ok(HttpResponse::Ok().json(user)),
			Err(_) => Ok(HttpResponse::InternalServerError().into()),
		})
		.responder()
}

fn main() {
	::std::env::set_var("RUST_LOG", "actix_web=info");
	env_logger::init();
	let sys = actix::System::new("diesel-web");
	let manager = ConnectionManager::<SqliteConnection>::new("test.db");
	let pool = r2d2::Pool::builder()
		.build(manager)
		.expect("Failed to build pool");
	let addr = SyncArbiter::start(3, move || DbExecutor(pool.clone()));

	server::new(move || {
		App::with_state(AppState { db: addr.clone() })
			.middleware(middleware::Logger::default())
			.resource("/{name}", |r| r.method(http::Method::GET).with(index))
	}).bind("127.0.0.1:8080")
		.unwrap()
		.start();

	let _ = sys.run();
}
