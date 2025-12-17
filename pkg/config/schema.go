package config

// Schema is the JSON schema for validating configuration files
const Schema = `{
    "$schema": "http://json-schema.org/draft-07/schema#",
    "type": "object",
    "properties": {
        "backup_dir": {
            "type": "string",
            "description": "Directory where backups will be stored"
        },
        "global_defaults": {
            "type": "object",
            "properties": {
                "port": {
                    "type": "integer",
                    "minimum": 1,
                    "maximum": 65535
                },
                "retention_tiers": {
                    "type": "array",
                    "items": {
                        "type": "object",
                        "properties": {
                            "tier": {
                                "type": "string",
                                "enum": ["hourly", "daily", "weekly", "monthly", "quarterly", "yearly"]
                            },
                            "retention": {
                                "type": "integer",
                                "minimum": 0
                            }
                        },
                        "required": ["tier", "retention"]
                    }
                },
                "pgpass_file": {
                    "type": "string"
                }
            }
        },
        "max_concurrent_backups": {
            "type": "integer",
            "minimum": 1
        },
        "log_level": {
            "type": "string",
            "enum": ["debug", "info", "warn", "error"]
        },
        "log_format": {
            "type": "string",
            "enum": ["json", "console"]
        },
        "databases": {
            "type": "array",
            "items": {
                "type": "object",
                "properties": {
                    "name": {
                        "type": "string",
                        "pattern": "^[a-zA-Z0-9_-]+$"
                    },
                    "user": {
                        "type": "string"
                    },
                    "host": {
                        "type": "string"
                    },
                    "port": {
                        "type": "integer",
                        "minimum": 1,
                        "maximum": 65535
                    },
                    "retention_tiers": {
                        "type": "array",
                        "items": {
                            "type": "object",
                            "properties": {
                                "tier": {
                                    "type": "string",
                                    "enum": ["hourly", "daily", "weekly", "monthly", "quarterly", "yearly"]
                                },
                                "retention": {
                                    "type": "integer",
                                    "minimum": 0
                                }
                            },
                            "required": ["tier", "retention"]
                        }
                    },
                    "enabled": {
                        "type": "boolean"
                    }
                },
                "required": ["name", "user", "host"]
            }
        }
    },
    "required": ["backup_dir", "databases"]
}`
