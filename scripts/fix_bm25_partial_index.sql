-- ============================================================================
-- 修复 BM25 部分索引脚本
-- 
-- 问题：paradedb.score() 函数错误
-- 错误信息：ERROR: function "pdb.score(anyelement)" does not exist (SQLSTATE 42883)
-- 触发场景：UPDATE embeddings SET is_enabled = false WHERE ...
--
-- 根本原因：
-- 1. BM25 索引不是部分索引，包含了所有记录（包括 is_enabled = false）
-- 2. UPDATE 操作触发 ParadeDB BM25 索引的重新计算
-- 3. 在重新计算过程中，ParadeDB 内部调用 pdb.score() 函数
-- 4. 如果记录状态不符合索引条件，函数调用失败
--
-- 解决方案：将 BM25 索引改为部分索引（只索引 is_enabled = true 的记录）
-- 优势：
-- - 只索引启用的记录，减少索引大小
-- - UPDATE is_enabled = false 时，记录自动从索引中移除
-- - 不会触发 pdb.score() 函数调用，避免错误
-- ============================================================================

DO $$
BEGIN
    -- 检查当前索引是否存在
    IF EXISTS (SELECT 1 FROM pg_indexes WHERE indexname = 'embeddings_search_idx') THEN
        RAISE NOTICE '找到现有索引 embeddings_search_idx，准备重建为部分索引...';
        
        -- 删除现有索引
        DROP INDEX IF EXISTS embeddings_search_idx;
        RAISE NOTICE '已删除现有 BM25 索引';
        
        -- 创建部分索引（只索引启用的记录）
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
        
        RAISE NOTICE '✅ 已创建部分 BM25 索引（只索引启用的记录）';
        RAISE NOTICE '   索引条件: WHERE (is_enabled IS NULL OR is_enabled = true)';
    ELSE
        RAISE NOTICE '索引 embeddings_search_idx 不存在，创建新的部分索引...';
        
        -- 创建部分索引
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
        
        RAISE NOTICE '✅ 已创建部分 BM25 索引';
    END IF;
    
    -- 验证索引创建
    IF EXISTS (
        SELECT 1 
        FROM pg_indexes 
        WHERE indexname = 'embeddings_search_idx' 
        AND indexdef LIKE '%WHERE%'
    ) THEN
        RAISE NOTICE '✅ 验证成功：索引是部分索引';
    ELSE
        RAISE WARNING '⚠️ 警告：索引可能不是部分索引，请检查';
    END IF;
END $$;

-- ============================================================================
-- 验证索引信息
-- ============================================================================
SELECT 
    schemaname,
    tablename,
    indexname,
    indexdef
FROM pg_indexes
WHERE indexname = 'embeddings_search_idx';

-- ============================================================================
-- 说明
-- ============================================================================
-- 部分索引的优势：
-- 1. 只索引 is_enabled = true 的记录，减少索引大小
-- 2. 避免对已禁用记录调用 paradedb.score() 函数时出错
-- 3. 提高查询性能（只搜索启用的记录）
--
-- 注意事项：
-- 1. 查询时必须包含条件：WHERE (is_enabled IS NULL OR is_enabled = true)
-- 2. 删除记录时，先设置 is_enabled = false，然后物理删除
-- 3. 这样可以避免 paradedb.score() 函数错误
