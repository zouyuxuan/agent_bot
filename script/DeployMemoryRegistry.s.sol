// SPDX-License-Identifier: MIT
pragma solidity ^0.8.26;

import "forge-std/Script.sol";
import "../contracts/MemoryRegistry.sol";

contract DeployMemoryRegistry is Script {
    function run() external returns (MemoryRegistry registry) {
        bytes32 deployerPrivateKey = vm.envBytes32("DEPLOYER_PRIVATE_KEY");
        uint256 privateKeyUint = uint256(deployerPrivateKey);
        vm.startBroadcast(privateKeyUint);
        registry = new MemoryRegistry();
        vm.stopBroadcast();
    }
}
