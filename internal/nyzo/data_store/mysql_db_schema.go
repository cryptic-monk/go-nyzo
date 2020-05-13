package data_store

const (
	createTransactionsTableStatement = `CREATE TABLE IF NOT EXISTS transactions (
		height BIGINT SIGNED NOT NULL,
		canonical INT SIGNED NOT NULL,
		timestamp BIGINT SIGNED NOT NULL,
		type TINYINT SIGNED NOT NULL,
		sender BINARY(32),
		recipient BINARY(32),
		data BINARY(32),
		amount BIGINT,
		amount_after_fees BIGINT,
		sender_balance BIGINT,
		recipient_balance BIGINT,
		signature BINARY(64),
		vote TINYINT,
		original_signature BINARY(64),
		INDEX(sender(4)),
		INDEX(recipient(4)),
		INDEX(timestamp),
		INDEX(signature(4)),
		INDEX(original_signature(4)),
		INDEX(type),
		PRIMARY KEY (height, canonical))`
	addTransactionStatement    = `REPLACE INTO transactions(height, canonical, timestamp, type, sender, recipient, data, amount, amount_after_fees, sender_balance, recipient_balance, signature, vote, original_signature) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	createBlocksTableStatement = `CREATE TABLE IF NOT EXISTS blocks (
		chain_version SMALLINT SIGNED NOT NULL,
		height BIGINT SIGNED NOT NULL,
		hash BINARY(32) NOT NULL,
		previous_hash BINARY(32) NOT NULL,
		cycle_length INT SIGNED NOT NULL,
		start BIGINT SIGNED NOT NULL,
		verification BIGINT SIGNED NOT NULL,
		transaction_count INT SIGNED NOT NULL,
		balance_list_hash BINARY(32) NOT NULL,
		verifier BINARY(32) NOT NULL,
		signature BINARY(64) NOT NULL,
		INDEX(hash(4)),
		INDEX(signature(4)),
		PRIMARY KEY (height))`
	addBlockStatement               = `REPLACE INTO blocks(chain_version, height, hash, previous_hash, cycle_length, start, verification, transaction_count, balance_list_hash, verifier, signature) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	createCycleEventsTableStatement = `CREATE TABLE IF NOT EXISTS cycle_events (
		height BIGINT SIGNED NOT NULL,
		identifier BINARY(32) NOT NULL,
		joined TINYINT NOT NULL,
		PRIMARY KEY (height, identifier))`
	addCycleEventStatement         = `REPLACE INTO cycle_events(height, identifier, joined) VALUES(?, ?, ?)`
	createNodeStatusTableStatement = `CREATE TABLE IF NOT EXISTS node_status (
		identifier BINARY(32) NOT NULL,
		timestamp BIGINT SIGNED NOT NULL,
		ip BINARY(4) NOT NULL,
		nickname VARCHAR(32) NOT NULL,
		version INT SIGNED NOT NULL,
		mesh_total INT SIGNED NOT NULL,
		mesh_cycle INT SIGNED NOT NULL,
		cycle_length INT SIGNED NOT NULL,
		transactions INT SIGNED NOT NULL,
		retention_edge BIGINT SIGNED NOT NULL,
		trailing_edge BIGINT SIGNED NOT NULL,
		frozen_edge BIGINT SIGNED NOT NULL,
		open_edge BIGINT SIGNED NOT NULL,
		blocks_transmitted BIGINT SIGNED NOT NULL,
		blocks_created BIGINT SIGNED NOT NULL,
		vote_stats VARCHAR(1024) NOT NULL,
		last_join_height BIGINT SIGNED NOT NULL,
		last_removal_height BIGINT SIGNED NOT NULL,
		receiving_udp TINYINT NOT NULL,
		PRIMARY KEY (ip, identifier, nickname)) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci`
	addNodeStatusStatement = `REPLACE INTO node_status(identifier, timestamp, ip, nickname, version, mesh_total, mesh_cycle, cycle_length, transactions, retention_edge, trailing_edge, frozen_edge, open_edge, blocks_transmitted, blocks_created, vote_stats, last_join_height, last_removal_height, receiving_udp) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	highestBlockStatement  = `SELECT COALESCE(MAX(height), 0) AS max_height FROM blocks`
)
