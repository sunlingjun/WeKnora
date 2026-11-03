-- 回滚「我的文档」表（手工执行；顺序：先子表后父表）

DROP TABLE IF EXISTS mydoc_mounts;
DROP TABLE IF EXISTS mydoc_tickets;
DROP TABLE IF EXISTS mydoc_shares;
DROP TABLE IF EXISTS mydoc_files;
DROP TABLE IF EXISTS mydoc_nodes;
