-- ============================================================================
-- 临时删除 BM25 索引，执行删除操作，然后重建索引
-- 这是一个临时解决方案，用于绕过 ParadeDB BM25 索引的 pdb.score() 错误
-- ============================================================================

DO $$
DECLARE
    knowledge_id_to_delete VARCHAR(36) := '301be2ba-f5ea-4be1-9ee3-aa520b25e4a9';  -- 替换为实际的知识库 ID
BEGIN
    RAISE NOTICE '开始临时删除索引并执行删除操作...';
    
    -- Step 1: 检查并删除 BM25 索引
    IF EXISTS (SELECT 1 FROM pg_indexes WHERE indexname = 'embeddings_search_idx') THEN
        RAISE NOTICE '删除 BM25 索引...';
        DROP INDEX IF EXISTS embeddings_search_idx;
        RAISE NOTICE '✅ BM25 索引已删除';
    ELSE
        RAISE NOTICE '⚠️ BM25 索引不存在';
    END IF;
    
    -- Step 2: 执行删除操作（现在不会触发 pdb.score() 错误）
    RAISE NOTICE '执行删除操作...';
    DELETE FROM embeddings WHERE knowledge_id = knowledge_id_to_delete;
    RAISE NOTICE '✅ 已删除知识库 % 的所有 embeddings 记录', knowledge_id_to_delete;
    
    -- Step 3: 重建 BM25 部分索引
    RAISE NOTICE '重建 BM25 部分索引...';
    CREATE INDEX embeddings_search_idx ON embeddings
    USING bm25 (id, knowledge_base_id, content, knowledge_id, chunk_id)
    WITH (
        key_field = 'id',
        text_fields = '{
            "content": {
              "tokenizer": {"type": "chinese_lindera"}
            }
        }'
    )
    WHERE (is_enabled IS NULL OR is_enabled = true);
    
    RAISE NOTICE '✅ BM25 部分索引已重建';
    RAISE NOTICE '操作完成！';
END $$;

-- ============================================================================
-- 验证删除结果
-- ============================================================================
SELECT 
    COUNT(*) AS remaining_records
FROM embeddings
WHERE knowledge_id = '301be2ba-f5ea-4be1-9ee3-aa520b25e4a9';

-- ============================================================================
-- 验证索引重建
-- ============================================================================
SELECT 
    indexname,
    indexdef
FROM pg_indexes
WHERE indexname = 'embeddings_search_idx';
