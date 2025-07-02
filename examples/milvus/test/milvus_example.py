from pymilvus import connections, FieldSchema, CollectionSchema, DataType, Collection, utility

# 1. Connect to Milvus
def connect_to_milvus():
    connections.connect("default", host="localhost", port="19530")
    print("Connected to Milvus.")

# 2. Create Collection
def create_collection(collection_name):
    fields = [
        FieldSchema(name="id", dtype=DataType.INT64, is_primary=True, auto_id=True),
        FieldSchema(name="embedding", dtype=DataType.FLOAT_VECTOR, dim=128)
    ]

    # define schema
    schema = CollectionSchema(fields, "example collection")

    # create collection
    collection = Collection(collection_name, schema)
    print(f"Collection '{collection_name}' created.")
    return collection

# 3. insert data
def insert_data(collection, num_entities):
    import numpy as np
    embeddings = np.random.random((num_entities, 128)).tolist()
    entities = [
        embeddings
    ]
    collection.insert(entities)
    print(f"Inserted {num_entities} entities into the collection.")

# 4. create index
def create_index(collection, field_name):
    index_params = {
        "index_type": "IVF_FLAT",
        "metric_type": "L2",
        "params": {"nlist": 128}
    }
    collection.create_index(field_name, index_params)
    print(f"Index created on field '{field_name}'.")

# 5. load collection
def load_collection(collection):
    collection.load()
    print("Collection loaded into memory.")

# 6. search
def simple_query(collection, query_vector):
    search_params = {
        "metric_type": "L2",
        "params": {"nprobe": 10}
    }
    results = collection.search(query_vector, "embedding", search_params, limit=10)
    print("Query results:")
    for hits in results:
        for hit in hits:
            print(f"Hit: {hit}, Distance: {hit.distance}")

# 7. drop collection
def drop_collection(collection_name):
    utility.drop_collection(collection_name)
    print(f"Collection '{collection_name}' dropped.")

def main():
    collection_name = "example_collection"

    connect_to_milvus()

    collection = create_collection(collection_name)

    insert_data(collection, 1000)

    create_index(collection, "embedding")

    load_collection(collection)

    import numpy as np
    query_vector = np.random.random((1, 128)).tolist()
    simple_query(collection, query_vector)

    drop_collection(collection_name)

if __name__ == "__main__":
    main()