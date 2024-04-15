---
title: Backup Encryption
description: How to encrypt backups
keywords: [encryption, backup, restore]
sidebar_position: 1
sidebar_label: Backup Encryption
---

# Backup Encryption

KubeBlocks has integrated data encryption functionality for datasafed since v0.9.0. Backup data is encrypted before being written to storage. You can create different secret keys for database clusters according to actual needs. Currently, the supported encryption algorithms include `AES-128-CFB`, `AES-192-CFB`, and `AES-256-CFB`.

1. Create a Secret to store the encryption key.

    ```bash
    kubectl create secret generic backup-encryption \
    --from-literal=secretKey='your secret key'
    ```

2. Patch the BackupPolicy to enable encryption. 
Remember to reference the key created before:

    ```bash
    kubectl --type merge patch backuppolicy mysqlcluster-mysql-backup-policy \
    -p '{"spec":{"encryptionConfig":{"algorithm":"AES-256-CFB","passPhraseSecretKeyRef":{"name":"backup-encryption","key":"secretKey"}}}}'
    ```

3. Complete. 
Now you can perform backups and restores as usual.

:::Note

The secret created in Step 1 should not be modified or deleted; otherwise, decryption of backups may fail.

:::