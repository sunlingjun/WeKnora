-- ============================================================================
-- 检查 embeddings 表的触发器和索引状态
-- ============================================================================

-- 1. 检查是否有触发器
SELECT 
    tgname AS trigger_name,
    tgtype::text AS trigger_type,
    tgenabled AS enabled,
    pg_get_triggerdef(oid) AS trigger_definition
FROM pg_trigger
WHERE tgrelid = 'embeddings'::regclass
ORDER BY tgname;

-- 2. 检查 BM25 索引的详细定义
SELECT 
    schemaname,
    tablename,
    indexname,
    indexdef
FROM pg_indexes
WHERE tablename = 'embeddings'
  AND indexname = 'embeddings_search_idx';

-- 3. 检查索引是否为部分索引
SELECT 
    i.indexname,
    i.indexdef,
    CASE 
        WHEN i.indexdef LIKE '%WHERE%' THEN 'Partial Index'
        ELSE 'Full Index'
    END AS index_type,
    pg_size_pretty(pg_relation_size(i.indexname::regclass)) AS index_size
FROM pg_indexes i
WHERE i.tablename = 'embeddings'
  AND i.indexname = 'embeddings_search_idx';

-- 4. 检查是否有其他 BM25 索引
SELECT 
    indexname,
    indexdef
FROM pg_indexes
WHERE tablename = 'embeddings'
  AND indexdef LIKE '%bm25%';

-- 5. 检查表的约束和规则
SELECT 
    conname AS constraint_name,
    contype AS constraint_type,
    pg_get_constraintdef(oid) AS constraint_definition
FROM pg_constraint
WHERE conrelid = 'embeddings'::regclass;

-- 6. 检查是否有规则（rules）
SELECT 
    rulename AS rule_name,
    pg_get_ruledef(oid) AS rule_definition
FROM pg_rules
WHERE tablename = 'embeddings';
