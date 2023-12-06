/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

This file is part of KubeBlocks project

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package mysql

import (
	"fmt"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"

	"github.com/apecloud/kubeblocks/pkg/lorry/engines"
	"github.com/apecloud/kubeblocks/pkg/lorry/engines/models"
)

var _ engines.ClusterCommands = &Commands{}

type Commands struct {
	info     engines.EngineInfo
	examples map[models.ClientType]engines.BuildConnectExample
}

func NewCommands() engines.ClusterCommands {
	return &Commands{
		info: engines.EngineInfo{
			Client:      "mysql",
			PasswordEnv: "$MYSQL_ROOT_PASSWORD",
			UserEnv:     "$MYSQL_ROOT_USER",
			Database:    "mysql",
		},
		examples: map[models.ClientType]engines.BuildConnectExample{
			models.CLI: func(info *engines.ConnectionInfo) string {
				return fmt.Sprintf(`# mysql client connection example
mysql -h %s -P %s -u %s -p%s
`, info.Host, info.Port, info.User, info.Password)
			},

			models.DJANGO: func(info *engines.ConnectionInfo) string {
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

			models.DOTNET: func(info *engines.ConnectionInfo) string {
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

			models.GO: func(info *engines.ConnectionInfo) string {
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

			models.JAVA: func(info *engines.ConnectionInfo) string {
				return fmt.Sprintf(`Class.forName("com.mysql.cj.jdbc.Driver");
Connection conn = DriverManager.getConnection(
  "jdbc:mysql://%s:%s/%s?sslMode=VERIFY_IDENTITY",
  "%s",
  "%s");
`, info.Host, info.Port, info.Database, info.User, info.Password)
			},

			models.NODEJS: func(info *engines.ConnectionInfo) string {
				return fmt.Sprintf(`# .env
DATABASE_URL='mysql://%s:%s@%s:%s/%s?ssl={"rejectUnauthorized":true}'

# app.js
require('dotenv').config();
const mysql = require('mysql2');
const connection = mysql.createConnection(process.env.DATABASE_URL);
connection.end();
`, info.User, info.Password, info.Host, info.Port, info.Database)
			},

			models.PHP: func(info *engines.ConnectionInfo) string {
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

			models.PRISMA: func(info *engines.ConnectionInfo) string {
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

			models.PYTHON: func(info *engines.ConnectionInfo) string {
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

			models.RAILS: func(info *engines.ConnectionInfo) string {
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

			models.RUST: func(info *engines.ConnectionInfo) string {
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

			models.SYMFONY: func(info *engines.ConnectionInfo) string {
				return fmt.Sprintf(`# .env
DATABASE_URL='mysql://%s:%s@%s:%s/%s'
`, info.User, info.Password, info.Host, info.Port, info.Database)
			},
		},
	}
}

func (m *Commands) ConnectCommand(connectInfo *engines.AuthInfo) []string {
	userName := m.info.UserEnv
	userPass := m.info.PasswordEnv

	if connectInfo != nil {
		userName = engines.AddSingleQuote(connectInfo.UserName)
		userPass = engines.AddSingleQuote(connectInfo.UserPasswd)
	}

	// avoid using env variables
	// MYSQL_PWD is deprecated as of MySQL 8.0; expect it to be removed in a future version of MySQL.
	// ref to mysql manual for more details.
	// https://dev.mysql.com/doc/refman/8.0/en/environment-variables.html
	mysqlCmd := []string{fmt.Sprintf("%s -u%s -p%s", m.info.Client, userName, userPass)}

	return []string{"sh", "-c", strings.Join(mysqlCmd, " ")}
}

func (m *Commands) Container() string {
	return m.info.Container
}

func (m *Commands) ConnectExample(info *engines.ConnectionInfo, client string) string {
	if len(info.Database) == 0 {
		info.Database = m.info.Database
	}
	return engines.BuildExample(info, client, m.examples)
}

func (m *Commands) ExecuteCommand(scripts []string) ([]string, []corev1.EnvVar, error) {
	cmd := []string{}
	cmd = append(cmd, "/bin/sh", "-c", "-ex")
	cmd = append(cmd, fmt.Sprintf("%s -u%s -p%s -e %s", m.info.Client,
		fmt.Sprintf("$%s", engines.EnvVarMap[engines.USER]),
		fmt.Sprintf("$%s", engines.EnvVarMap[engines.PASSWORD]),
		strconv.Quote(strings.Join(scripts, " "))))

	envs := []corev1.EnvVar{
		{
			Name:  "MYSQL_HOST",
			Value: fmt.Sprintf("$(%s)", engines.EnvVarMap[engines.HOST]),
		},
	}
	return cmd, envs, nil
}
