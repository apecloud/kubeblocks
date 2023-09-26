output: {
	"filelog/mysql/error": {
		type:
		include: [ "/data/mysql/log/mysqld-error.log"]
		include_file_name: false
		start_at: "beginning"
	}

	"filelog/mysql/slow":{
		type: "slow"
		include:[ "/data/mysql/log/mysqld-slowquery.log"]
		include_file_name: false
		start_at: "beginning"
	}
}
