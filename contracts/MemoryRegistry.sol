// SPDX-License-Identifier: MIT
pragma solidity ^0.8.26;

contract MemoryRegistry {
    enum AssetKind {
        TrainingMemory,
        DistilledMemory
    }

    struct AssetRecord {
        bytes32 assetId;
        address owner;
        AssetKind kind;
        bytes32 rootHash;
        string storageRef;
        string name;
        bytes32 parentAssetId;
        uint64 createdAt;
    }

    mapping(bytes32 => AssetRecord) private assets;
    mapping(address => bytes32[]) private ownerAssets;

    event AssetRegistered(
        bytes32 indexed assetId,
        address indexed owner,
        uint8 indexed kind,
        bytes32 rootHash,
        string storageRef,
        bytes32 parentAssetId,
        string name
    );

    error InvalidKind();
    error EmptyRootHash();
    error EmptyStorageRef();
    error EmptyName();
    error AssetAlreadyRegistered(bytes32 assetId);
    error AssetNotFound(bytes32 assetId);

    function previewAssetId(
        address owner,
        uint8 kind,
        bytes32 rootHash,
        string calldata storageRef,
        string calldata name,
        bytes32 parentAssetId
    ) public pure returns (bytes32) {
        return keccak256(abi.encode(owner, kind, rootHash, storageRef, name, parentAssetId));
    }

    function registerAsset(
        uint8 kind,
        bytes32 rootHash,
        string calldata storageRef,
        string calldata name,
        bytes32 parentAssetId
    ) external returns (bytes32 assetId) {
        _validate(kind, rootHash, storageRef, name);
        assetId = previewAssetId(msg.sender, kind, rootHash, storageRef, name, parentAssetId);
        if (assets[assetId].owner != address(0)) {
            revert AssetAlreadyRegistered(assetId);
        }

        AssetRecord memory record = AssetRecord({
            assetId: assetId,
            owner: msg.sender,
            kind: AssetKind(kind),
            rootHash: rootHash,
            storageRef: storageRef,
            name: name,
            parentAssetId: parentAssetId,
            createdAt: uint64(block.timestamp)
        });

        assets[assetId] = record;
        ownerAssets[msg.sender].push(assetId);

        emit AssetRegistered(assetId, msg.sender, kind, rootHash, storageRef, parentAssetId, name);
    }

    function getAsset(bytes32 assetId) external view returns (AssetRecord memory) {
        AssetRecord memory record = assets[assetId];
        if (record.owner == address(0)) {
            revert AssetNotFound(assetId);
        }
        return record;
    }

    function getAssetsByOwner(address owner) external view returns (bytes32[] memory) {
        return ownerAssets[owner];
    }

    function _validate(uint8 kind, bytes32 rootHash, string calldata storageRef, string calldata name) private pure {
        if (kind > uint8(AssetKind.DistilledMemory)) {
            revert InvalidKind();
        }
        if (rootHash == bytes32(0)) {
            revert EmptyRootHash();
        }
        if (bytes(storageRef).length == 0) {
            revert EmptyStorageRef();
        }
        if (bytes(name).length == 0) {
            revert EmptyName();
        }
    }
}
