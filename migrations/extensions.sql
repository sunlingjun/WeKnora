-- ============================================
-- WeKnora 数据库扩展安装脚本
-- ============================================
-- 说明：此脚本用于安装 WeKnora 项目所需的所有 PostgreSQL 扩展
-- 适用数据库：PostgreSQL / ParadeDB
-- 
-- 使用方法：
--   1. 以超级用户身份连接数据库
--   2. 执行此脚本：\i migrations/extensions.sql
--   或：psql -U postgres -d weknora -f migrations/extensions.sql
-- ============================================

-- 检查当前数据库
DO $$
BEGIN
    RAISE NOTICE '============================================';
    RAISE NOTICE 'WeKnora 数据库扩展安装脚本';
    RAISE NOTICE '数据库: %', current_database();
    RAISE NOTICE 'PostgreSQL 版本: %', version();
    RAISE NOTICE '============================================';
END $$;

-- ============================================
-- 1. uuid-ossp 扩展（必需）
-- ============================================
-- 用途：UUID 生成
-- 适用：所有环境
DO $$
BEGIN
    CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
    RAISE NOTICE '✅ uuid-ossp 扩展已安装';
EXCEPTION
    WHEN OTHERS THEN
        RAISE WARNING '❌ uuid-ossp 扩展安装失败: %', SQLERRM;
END $$;

-- ============================================
-- 2. vector 扩展（必需）
-- ============================================
-- 用途：向量相似度搜索（pgvector）
-- 适用：PostgreSQL / ParadeDB
-- 注意：需要 pgvector >= 0.5.0（支持 halfvec）
DO $$
BEGIN
    CREATE EXTENSION IF NOT EXISTS vector;
    RAISE NOTICE '✅ vector 扩展已安装';
EXCEPTION
    WHEN OTHERS THEN
        RAISE WARNING '❌ vector 扩展安装失败: %', SQLERRM;
        RAISE WARNING '   提示：请确保已安装 pgvector >= 0.5.0';
END $$;

-- ============================================
-- 3. pg_trgm 扩展（必需）
-- ============================================
-- 用途：文本相似度搜索（Trigram）
-- 适用：PostgreSQL / ParadeDB
DO $$
BEGIN
    CREATE EXTENSION IF NOT EXISTS pg_trgm;
    RAISE NOTICE '✅ pg_trgm 扩展已安装';
EXCEPTION
    WHEN OTHERS THEN
        RAISE WARNING '❌ pg_trgm 扩展安装失败: %', SQLERRM;
        RAISE WARNING '   提示：请确保已安装 postgresql-contrib 包';
END $$;

-- ============================================
-- 4. pg_search 扩展（ParadeDB 专用）
-- ============================================
-- 用途：全文搜索（BM25）
-- 适用：仅 ParadeDB
-- 注意：标准 PostgreSQL 不支持此扩展
DO $$
BEGIN
    CREATE EXTENSION IF NOT EXISTS pg_search;
    RAISE NOTICE '✅ pg_search 扩展已安装（ParadeDB）';
EXCEPTION
    WHEN OTHERS THEN
        RAISE WARNING '⚠️  pg_search 扩展安装失败: %', SQLERRM;
        RAISE WARNING '   提示：此扩展仅 ParadeDB 支持，标准 PostgreSQL 可忽略此错误';
END $$;

-- ============================================
-- 验证安装结果
-- ============================================
DO $$
DECLARE
    ext_record RECORD;
    installed_count INTEGER := 0;
BEGIN
    RAISE NOTICE '';
    RAISE NOTICE '============================================';
    RAISE NOTICE '扩展安装验证';
    RAISE NOTICE '============================================';
    
    FOR ext_record IN 
        SELECT extname, extversion 
        FROM pg_extension 
        WHERE extname IN ('uuid-ossp', 'vector', 'pg_trgm', 'pg_search')
        ORDER BY extname
    LOOP
        RAISE NOTICE '  ✅ % (版本: %)', ext_record.extname, ext_record.extversion;
        installed_count := installed_count + 1;
    END LOOP;
    
    IF installed_count = 0 THEN
        RAISE WARNING '  ⚠️  未检测到任何扩展，请检查安装过程';
    ELSIF installed_count < 3 THEN
        RAISE WARNING '  ⚠️  部分扩展未安装，请检查上述错误信息';
    ELSE
        RAISE NOTICE '  ✅ 所有必需扩展已安装';
    END IF;
    
    RAISE NOTICE '============================================';
END $$;

-- ============================================
-- 功能测试
-- ============================================
DO $$
BEGIN
    RAISE NOTICE '';
    RAISE NOTICE '============================================';
    RAISE NOTICE '功能测试';
    RAISE NOTICE '============================================';
    
    -- 测试 uuid-ossp
    BEGIN
        PERFORM uuid_generate_v4();
        RAISE NOTICE '  ✅ uuid-ossp: UUID 生成功能正常';
    EXCEPTION
        WHEN OTHERS THEN
            RAISE WARNING '  ❌ uuid-ossp: UUID 生成功能异常';
    END;
    
    -- 测试 pg_trgm
    BEGIN
        PERFORM similarity('hello', 'hallo');
        RAISE NOTICE '  ✅ pg_trgm: 文本相似度计算功能正常';
    EXCEPTION
        WHEN OTHERS THEN
            RAISE WARNING '  ❌ pg_trgm: 文本相似度计算功能异常';
    END;
    
    -- 测试 vector（如果已安装）
    BEGIN
        IF EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'vector') THEN
            RAISE NOTICE '  ✅ vector: 扩展已安装（功能测试需要实际向量数据）';
        END IF;
    EXCEPTION
        WHEN OTHERS THEN
            RAISE WARNING '  ❌ vector: 扩展检查异常';
    END;
    
    RAISE NOTICE '============================================';
    RAISE NOTICE '扩展安装脚本执行完成！';
    RAISE NOTICE '============================================';
END $$;
