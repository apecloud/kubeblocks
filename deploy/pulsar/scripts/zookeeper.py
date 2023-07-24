#!/usr/bin/python3
from kazoo.client import KazooClient
from kazoo.exceptions import NodeExistsError, NoNodeError
import os
import sys
import logging

logging.basicConfig(level=logging.INFO,
                    format='%(asctime)s %(filename)s[line:%(lineno)d] %(levelname)s %(message)s',
                    datefmt='%a, %d %b %Y %H:%M:%S', )
logger = logging.getLogger()


class ZKClient(object):
    def __init__(self, zk_url):
        self.client = KazooClient(hosts=zk_url)
        self.client.start()

    def create_node(self, path, data=None):
        try:
            self.client.create(path, data.encode() if data else None)
            logger.info(f"Node {path} created successfully.")
        except NodeExistsError:
            self.stop_with_error(f"Node {path} already exists.")
        except Exception as e:
            self.stop_with_error(f"Failed to create node {path}: {e}")

    def get_node(self, path):
        try:
            data, _ = self.client.get(path)
            logger.info(f"Data at {path}: {data.decode()}")
        except NoNodeError:
            self.stop_with_error(f"Node {path} does not exist.")
        except Exception as e:
            self.stop_with_error(f"Failed to read node {path}: {e}")

    def update_node(self, path, data):
        try:
            self.client.set(path, data.encode())
            logger.info(f"Node {path} updated successfully.")
        except NoNodeError:
            self.stop_with_error(f"Node {path} does not exist.")
        except Exception as e:
            self.stop_with_error(f"Failed to update node {path}: {e}")

    def delete_node(self, path):
        try:
            self.client.delete(path)
            logger.info(f"Node {path} deleted successfully.")
        except NoNodeError:
            self.stop_with_error(f"Node {path} does not exist.")
        except Exception as e:
            self.stop_with_error(f"Failed to delete node {path}: {e}")

    def stop(self):
        self.client.stop()

    def stop_with_error(self, message):
        logger.error(message)
        self.client.stop()
        exit(1)


if __name__ == '__main__':
    args = sys.argv[1:]
    if len(args) != 2:
        raise Exception("requires 2 arguments.")
    op = args[0]
    path = args[1]
    zk_url = os.environ.get('zkURL')
    zk_client = ZKClient(zk_url)
    if op == "delete":
        zk_client.delete_node(path)
    elif op == "create":
        zk_client.create_node(path)
    elif op == "get":
        zk_client.get_node(path)
    else:
        zk_client.stop_with_error(f"Unknown operation: {op}")
    zk_client.stop()
