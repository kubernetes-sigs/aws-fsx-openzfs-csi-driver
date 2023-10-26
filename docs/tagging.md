# Tagging
The FSx for OpenZFS CSI driver automatically configures particular tags on FSx resources, in addition to user-provided tags.

This includes a general tag added to all dynamically created FSx resources, 
as well as deletion tags that are utilized to support deletion options.

## Driver Tags

The FSx for OpenZFS CSI driver will automatically add a unique tag to denote the resources it creates.
This tag is not used by the driver and is only provided to aid the user in resource management.
You can find all resources created by the OpenZFS CSI driver using this [hack](../hack/print-resources).

The CSI driver attaches the following tag to the resources it creates:

| Key                             | Value                       | Description                                               |
|---------------------------------|-----------------------------|-----------------------------------------------------------|
| fsx.openzfs.csi.aws.com/cluster | true                        | Attached to all FSx OpenZFS resources the driver creates. |                                                                                      |

## Deletion Tags

To get around CSI limitations, the driver also tags deletion parameters that were provided by the user.
Any parameter specified with the key suffix of `OnDeletion` is individually saved as a tag on the respective resource.

It is possible to modify the delete behavior of a resource after it's created due to this design.
Any parameter additions or modifications must be manually done using our API.
If tags are incorrect formatted when edited, they will be ignored during delete, and default behavior will take precedence.

The parameter(s) will still be saved as Key-Value pair(s).
The `OnDeletion` suffix will remain on the tag key for consistency.
Due to restrictions in tags, special characters used in JSON such as `",[]{}` are encoded.

Example Storage Class:

| Key                         | Value                                      |
|-----------------------------|--------------------------------------------|
| `SkipFinalBackupOnDeletion` | `'true'`                                   |
| `OptionsOnDeletion`         | `'["DELETE_CHILD_VOLUMES_AND_SNAPSHOTS"]'` |

Example Tag Representations:

| Key                         | Value                                            |
|-----------------------------|--------------------------------------------------|
| `SkipFinalBackupOnDeletion` | `true`                                           |
| `OptionsOnDeletion`         | ` -  @ DELETE_CHILD_VOLUMES_AND_SNAPSHOTS @  _ ` |
**Note: There is a space before and after the value of OptionsOnDeletion**

The following chart contains the translation between JSON characters and their respective tag representation.
To prevent mistranslation when decoding parameters with special characters, a space is added before and after the tag representations.

| JSON Character | Tag Character |
|----------------|---------------|
| `"`            | ` @ `         |
| `,`            | ` . `         |
| `[`            | ` - `         |
| `]`            | ` _ `         |
| `{`            | ` + `         |
| `}`            | ` = `         |
**Note: Tag Characters are proceeded and succeeded by a space**
