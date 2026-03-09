CREATE TABLE IF NOT EXISTS dogs (
    id UUID,
    name String,
    species String,
    breed String,
    age UInt8,
    weight_kg Float32,
    ingested_at DateTime DEFAULT now()
) ENGINE = MergeTree()
ORDER BY (breed, name);
