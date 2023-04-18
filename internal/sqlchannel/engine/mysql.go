/*
Copyright ApeCloud, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package engine

import (
	"fmt"
	"strings"
)

type mysql struct {
	info     EngineInfo
	examples map[ClientType]buildConnectExample
}

var _ Interface = &mysql{}

func newMySQL() *mysql {
	return &mysql{
		info: EngineInfo{
			Client:      "mysql",
			Container:   "mysql",
			PasswordEnv: "$MYSQL_ROOT_PASSWORD",
			UserEnv:     "$MYSQL_ROOT_USER",
			Database:    "mysql",
		},
		examples: map[ClientType]buildConnectExample{
			CLI: func(info *ConnectionInfo) string {
				return fmt.Sprintf(`# mysql client connection example
mysql -h %s -P %s -u %s -p%s
`, info.Host, info.Port, info.User, info.Password)
			},

			DJANGO: func(info *ConnectionInfo) string {
				return fmt.Sprintf(`# .env
DB_HOST=%s
DB_NAME=%s
DB_USER=%s
DB_PASSWORD=%s
DB_PORT=%s

# settings.py
DATABASES = {
  'default': {
    'ENGINE': 'django.db.backends.mysql',
    'NAME': os.environ.get('DB_NAME'),
    'HOST': os.environ.get('DB_HOST'),
    'PORT': os.environ.get('DB_PORT'),
    'USER': os.environ.get('DB_USER'),
    'PASSWORD': os.environ.get('DB_PASSWORD'),
  }
}
`, info.Host, info.Database, info.User, info.Password, info.Port)
			},

			DOTNET: func(info *ConnectionInfo) string {
				return fmt.Sprintf(`# appsettings.json
{
  "ConnectionStrings": {
    "Default": "server=%s;port=%s;database=%s;user=%s;password=%s;SslMode=VerifyFull;"
  },
}

# Startup.cs
public void ConfigureServices(IServiceCollection services)
{
    services.AddTransient<MySqlConnection>(_ => new MySqlConnection(Configuration["ConnectionStrings:Default"]));
}
`, info.Host, info.Port, info.Database, info.User, info.Password)
			},

			GO: func(info *ConnectionInfo) string {
				const goConnectExample = `# main.go
package main

import (
    "database/sql"
    "log"
    "os"

     _ "github.com/go-sql-driver/mysql"
)

func main() {
    db, err := sql.Open("mysql", os.Getenv("DSN"))
    if err != nil {
        log.Fatalf("failed to connect: %v", err)
    }
    defer db.Close()

    if err := db.Ping(); err != nil {
        log.Fatalf("failed to ping: %v", err)
    }

    log.Println("Successfully connected!")
}
`
				dsn := fmt.Sprintf(`# .env
DSN=%s:%s@tcp(%s:%s)/%s?tls=true
`, info.User, info.Password, info.Host, info.Port, info.Database)
				return fmt.Sprintf("%s\n%s", dsn, goConnectExample)
			},

			JAVA: func(info *ConnectionInfo) string {
				return fmt.Sprintf(`Class.forName("com.mysql.cj.jdbc.Driver");
Connection conn = DriverManager.getConnection(
  "jdbc:mysql://%s:%s/%s?sslMode=VERIFY_IDENTITY",
  "%s",
  "%s");
`, info.Host, info.Port, info.Database, info.User, info.Password)
			},

			NODEJS: func(info *ConnectionInfo) string {
				return fmt.Sprintf(`# .env
DATABASE_URL='mysql://%s:%s@%s:%s/%s?ssl={"rejectUnauthorized":true}'

# app.js
require('dotenv').config();
const mysql = require('mysql2');
const connection = mysql.createConnection(process.env.DATABASE_URL);
connection.end();
`, info.User, info.Password, info.Host, info.Port, info.Database)
			},

			PHP: func(info *ConnectionInfo) string {
				return fmt.Sprintf(`# .env
HOST=%s
PORT=%s
USERNAME=%s
PASSWORD=%s
DATABASE=%s

# index.php
<?php
  $mysqli = mysqli_init();
  $mysqli->real_connect($_ENV["HOST"], $_ENV["USERNAME"], $_ENV["PASSWORD"], $_ENV["DATABASE"], $_ENV["PORT"]);
  $mysqli->close();
?>
`, info.Host, info.Port, info.User, info.Password, info.Database)
			},

			PRISMA: func(info *ConnectionInfo) string {
				return fmt.Sprintf(`# .env
DATABASE_URL='mysql://%s:%s@%s:%s/%s?sslaccept=strict'

# schema.prisma
generator client {
  provider = "prisma-client-js"
}

datasource db {
  provider = "mysql"
  url = env("DATABASE_URL")
  relationMode = "prisma"
}
`, info.User, info.Password, info.Host, info.Port, info.Database)
			},

			PYTHON: func(info *ConnectionInfo) string {
				return fmt.Sprintf(`# run the following command in the terminal to install dependencies
pip install python-dotenv mysqlclient

# .env
HOST=%s
PORT=%s
USERNAME=%s
PASSWORD=%s
DATABASE=%s

# main.py
from dotenv import load_dotenv
load_dotenv()
import os
import MySQLdb

connection = MySQLdb.connect(
  host= os.getenv("HOST"),
  port=os.getenv("PORT"),
  user=os.getenv("USERNAME"),
  passwd=os.getenv("PASSWORD"),
  db=os.getenv("DATABASE"),
  ssl_mode = "VERIFY_IDENTITY",
)
`, info.Host, info.Port, info.User, info.Password, info.Database)
			},

			RAILS: func(info *ConnectionInfo) string {
				return fmt.Sprintf(`# Gemfile
gem 'mysql2'

# config/database.yml
development:
  <<: *default
  adapter: mysql2
  database: %s
  username: %s
  host: %s
  password: %s
  ssl_mode: verify_identity
`, info.Database, info.User, info.Host, info.Password)
			},

			RUST: func(info *ConnectionInfo) string {
				return fmt.Sprintf(`# run the following command in the terminal
export DATABASE_URL="mysql://%s:%s@%s:%s/%s"

# src/main.rs
use std::env;

fn main() {
    let url = env::var("DATABASE_URL").expect("DATABASE_URL not found");
    let builder = mysql::OptsBuilder::from_opts(mysql::Opts::from_url(&url).unwrap());
    let pool = mysql::Pool::new(builder.ssl_opts(mysql::SslOpts::default())).unwrap();
    let _conn = pool.get_conn().unwrap();
    println!("Successfully connected!");
}

# Cargo.toml
[package]
name = "kubeblocks_hello_world"
version = "0.0.1"

[dependencies]
mysql = "*"
`, info.User, info.Password, info.Host, info.Port, info.Database)
			},

			SYMFONY: func(info *ConnectionInfo) string {
				return fmt.Sprintf(`# .env
DATABASE_URL='mysql://%s:%s@%s:%s/%s'
`, info.User, info.Password, info.Host, info.Port, info.Database)
			},
		},
	}
}

func (m *mysql) ConnectCommand(connectInfo *AuthInfo) []string {
	userName := m.info.UserEnv
	userPass := m.info.PasswordEnv

	if connectInfo != nil {
		userName = connectInfo.UserName
		userPass = connectInfo.UserPasswd
	}

	// avoid use env variables
	// MYSQL_PWD is deprecated as of MySQL 8.0; expect it to be removed in a future version of MySQL.
	// ref to mysql manual for more details.
	// https://dev.mysql.com/doc/refman/8.0/en/environment-variables.html
	mysqlCmd := []string{fmt.Sprintf("%s -u%s -p%s", m.info.Client, userName, userPass)}

	return []string{"sh", "-c", strings.Join(mysqlCmd, " ")}
}

func (m *mysql) Container() string {
	return m.info.Container
}

func (m *mysql) ConnectExample(info *ConnectionInfo, client string) string {
	if len(info.Database) == 0 {
		info.Database = m.info.Database
	}
	return buildExample(info, client, m.examples)
}
