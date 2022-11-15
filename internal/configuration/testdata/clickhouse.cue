#ProfilesParameter: {
    profiles: [string]: #ClickhouseParameter

    #ClickhouseParameter: {
        // [0|1|2] default 0
        readonly: int & 0 | 1 | 2 | *0

        // [0|1] default 1
        allow_ddl: int & 0 | 1 | *1

        // [deny|local|global|allow] default : deny
        distributed_product_mode: string & "deny" | "local" | "global" | "allow" | *"deny"

        // [0|1] default 0
        prefer_global_in_and_join: int & 0 | 1 | *0
        ...

        // other parameter
        // Clickhouse all parameter define: clickhouse settings define
    }

    // ignore other configure
}

configuration: #ProfilesParameter & {
}