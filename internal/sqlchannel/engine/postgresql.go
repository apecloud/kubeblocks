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

type postgresql struct {
	info     EngineInfo
	examples map[ClientType]buildConnectExample
}

var _ Interface = &postgresql{}

func newPostgreSQL() *postgresql {
	return &postgresql{
		info: EngineInfo{
			Client:      "psql",
			Container:   "postgresql",
			PasswordEnv: "$PGPASSWORD",
			UserEnv:     "$PGUSER",
			Database:    "postgres",
		},
		examples: map[ClientType]buildConnectExample{
			CLI: func(info *ConnectionInfo) string {
				return fmt.Sprintf(`# psql client connection example
PGPASSWORD=%s psql -h%s -p %s -U %s %s
`, info.Password, info.Host, info.Port, info.User, info.Database)
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
    'ENGINE': 'django.db.backends.postgresql',
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
				return fmt.Sprintf(`# Startup.cs
var connectionString = "Host=%s;Port=%s;Username=%s;Password=%s;Database=%s";
await using var dataSource = NpgsqlDataSource.Create(connectionString);
`, info.Host, info.Port, info.User, info.Password, info.Database)
			},

			GO: func(info *ConnectionInfo) string {
				const goConnectExample = `# main.go
package main

import (
    "database/sql"
    "log"
    "os"

     _ "github.com/lib/pq"
)

func main() {
    db, err := sql.Open("postgres", os.Getenv("DSN"))
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
DSN=%s:%s@tcp(%s:%s)/%s
`, info.User, info.Password, info.Host, info.Port, info.Database)
				return fmt.Sprintf("%s\n%s", dsn, goConnectExample)
			},

			JAVA: func(info *ConnectionInfo) string {
				return fmt.Sprintf(`Class.forName("org.postgresql.Driver");
Connection conn = DriverManager.getConnection(
  "jdbc:postgresql://%s:%s/%s?user=%s&password=%s");
`, info.Host, info.Port, info.Database, info.User, info.Password)
			},

			NODEJS: func(info *ConnectionInfo) string {
				return fmt.Sprintf(`# .env
DATABASE_URL='postgres://%s:%s@%s:%s/%s'

# app.js
require('dotenv').config();
const postgres = require('postgres');
const connection = postgres(process.env.DATABASE_URL);
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
  $dbconn =pg_connect($_ENV["HOST"], $_ENV["USERNAME"], $_ENV["PASSWORD"], $_ENV["DATABASE"], $_ENV["PORT"]);
  pg_close($dbconn)
?>
`, info.Host, info.Port, info.User, info.Password, info.Database)
			},

			PRISMA: func(info *ConnectionInfo) string {
				return fmt.Sprintf(`# .env
DATABASE_URL='postgres://%s:%s@%s:%s/%s'

# schema.prisma
generator client {
  provider = "prisma-client-js"
}

datasource db {
  provider = "postgresql"
  url = env("DATABASE_URL")
  relationMode = "prisma"
}
`, info.User, info.Password, info.Host, info.Port, info.Database)
			},

			PYTHON: func(info *ConnectionInfo) string {
				return fmt.Sprintf(`# run the following command in the terminal to install dependencies
pip install python-dotenv psycopg2

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

connection = psycopg2.connect(
  host= os.getenv("HOST"),
  port=os.getenv("PORT"),
  user=os.getenv("USERNAME"),
  password=os.getenv("PASSWORD"),
  database=os.getenv("DATABASE"),
)
`, info.Host, info.Port, info.User, info.Password, info.Database)
			},

			RAILS: func(info *ConnectionInfo) string {
				return fmt.Sprintf(`# Gemfile
gem 'pg'

# config/database.yml
development:
  <<: *default
  adapter: postgresql
  database: %s
  username: %s
  host: %s
  password: %s
`, info.Database, info.User, info.Host, info.Password)
			},

			RUST: func(info *ConnectionInfo) string {
				return fmt.Sprintf(`# run the following command in the terminal
export DATABASE_URL="postgresql://%s:%s@%s:%s/%s"

# src/main.rs
use std::env;

fn main() {
    let url = env::var("DATABASE_URL").expect("DATABASE_URL not found");
	let conn = Connection::connect(url, TlsMode::None).unwrap();
    println!("Successfully connected!");
}

# Cargo.toml
[package]
name = "kubeblocks_hello_world"
version = "0.0.1"
`, info.User, info.Password, info.Host, info.Port, info.Database)
			},

			SYMFONY: func(info *ConnectionInfo) string {
				return fmt.Sprintf(`# .env
DATABASE_URL='postgresql://%s:%s@%s:%s/%s'
`, info.User, info.Password, info.Host, info.Port, info.Database)
			},
		},
	}
}

func (m *postgresql) ConnectCommand(connectInfo *AuthInfo) []string {
	userName := m.info.UserEnv
	userPass := m.info.PasswordEnv

	if connectInfo != nil {
		userName = connectInfo.UserName
		userPass = connectInfo.UserPasswd
	}

	// pls refer to PostgreSQL documentation for more details
	// https://www.postgresql.org/docs/current/libpq-envars.html
	cmd := []string{fmt.Sprintf("PGUSER=%s PGPASSWORD=%s PGDATABASE=%s %s", userName, userPass, m.info.Database, m.info.Client)}
	return []string{"sh", "-c", strings.Join(cmd, " ")}
}

func (m *postgresql) Container() string {
	return m.info.Container
}

func (m *postgresql) ConnectExample(info *ConnectionInfo, client string) string {
	if len(info.Database) == 0 {
		info.Database = m.info.Database
	}
	return buildExample(info, client, m.examples)
}
