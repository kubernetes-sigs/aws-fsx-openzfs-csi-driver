# Tagging
Alongside tags a user defines, the CSI driver adds additional tags.
The types of tags the CSI driver adds includes driver and deletion tags.

## Driver Tags

The FSx for OpenZFS CSI driver will automatically add unique tag(s) to denote the resources it manages.
These tags are not used by the driver, and is only for aiding the user in resource management.

The CSI driver attaches the following tag(s) to resources created:

| Key                             | Value                       | Description                                     |
|---------------------------------|-----------------------------|-------------------------------------------------|
| fsx.openzfs.csi.aws.com/cluster | true                        | Attached to all FSx OpenZFS resources created.  |                                                                                      |

## Deletion Tags

The CSI driver also adds deletion parameters that were provided by the user.
Due to CSI limitations, parameters can't be defined when deleting resources.
To get around this, any parameter specified with the `OnDeletion` suffix is saved as a Key-Value pair.

Due to restriction in tags, special characters used in JSON such as `",[]{}` must be encoded.
The following chart contains the translation between JSON characters and their respective tag representation.
To prevent mistranslation when decoding parameters that contain translate characters, a space is added before and after the tag representation.

| JSON Character | Tag Character |
|----------------|---------------|
| `"`            | ` @ `         |
| `,`            | ` . `         |
| `[`            | ` - `         |
| `]`            | ` _ `         |
| `{`            | ` + `         |
| `}`            | ` = `         |
**Note: Tag Characters are proceeded and succeeded by a space**

#### Example:

JSON Representation:
```json
[{"Key": "OPENZFS", "Value": "OPENZFS"}]
```
Tag Representation:
```
 -  +  @ Key @ :  @ OPENZFS @  .  @ Value @ :  @ OPENZFS @  =  _ 
```

Deletion tags may be manually modified or added to change the delete behavior for a specific resource.
If tags are incorrect during delete they are ignored and the default behavior occurs.
The `OnDeletion` suffix remains on the tag key for parsing purposes.
