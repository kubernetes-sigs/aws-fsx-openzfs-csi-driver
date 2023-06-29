# Tagging
The FSx for OpenZFS CSI driver automatically configures particular tags on FSx resources, in addition to user-provided tags.

This includes a general tag added to all dynamically created FSx resources, 
as well as deletion tags that are utilized to support deletion options.

## Driver Tags

The FSx for OpenZFS CSI driver will automatically add a unique tag to denote the resources it creates.
This tag is not used by the driver and is only provided to aid the user in resource management.

The CSI driver attaches the following tag to the resources it creates:

| Key                             | Value                       | Description                                               |
|---------------------------------|-----------------------------|-----------------------------------------------------------|
| fsx.openzfs.csi.aws.com/cluster | true                        | Attached to all FSx OpenZFS resources the driver creates. |                                                                                      |

## Deletion Tags

The CSI driver also adds deletion parameters that were provided by the user.
Due to CSI limitations, parameters cannot be defined when deleting resources.
To get around this, any parameter specified with the `OnDeletion` suffix is saved as a tag on the resource via a Key-Value pair.

Due to restrictions in tags, special characters used in JSON such as `",[]{}` must be encoded.
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
["DELETE_CHILD_VOLUMES_AND_SNAPSHOTS"]
```
Tag Representation:
```
- @ DELETE_CHILD_VOLUMES_AND_SNAPSHOTS @ _
```

Deletion tags may be manually modified or added to change the delete behavior for a specific resource.
If tags are incorrect during delete they are ignored and the default behavior occurs.
The `OnDeletion` suffix remains on the tag key for parsing purposes.
