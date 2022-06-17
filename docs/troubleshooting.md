## Troubleshooting

### Known Issues:

- As of now, the CSI plugin blocks the restore process for each PVC that will
    be restored via volumesnapshotmover restore. 
    - This can create long restore times, so we are working on possible solutions.
    