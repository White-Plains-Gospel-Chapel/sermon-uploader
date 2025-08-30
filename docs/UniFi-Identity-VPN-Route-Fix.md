# UniFi Identity VPN Route Fix Documentation

## Overview

This guide addresses an issue where a Mac connected to UniFi Identity One-Click VPN (using a 10.23.100.x tunnel IP) cannot access devices on the home LAN (e.g., 192.168.1.x, such as a Raspberry Pi at 192.168.1.127) due to routing conflicts, especially with split tunneling enabled. The local network's subnet overlap (192.168.1.x) causes the Mac to prioritize the local WiFi interface (`en0`) over the VPN interface (`utun6`). This document provides a manual routing fix and additional context for future reference.

## Problem

- **Symptoms**: VPN connection succeeds, but LAN devices (e.g., 192.168.1.127) are unreachable.
- **Cause**: The routing table shows `192.168.1.0/24` routed via `en0` (local WiFi) instead of `utun6` (VPN), likely due to subnet overlap or split tunneling excluding the LAN from the VPN tunnel.
- **Example Routing Table Entry**:
    
    ```
    192.168.1.0/24    link#14    UCS    en0    !
    192.168.1.180      UHWi       utun6  1195
    ```
    

## Solution: Manual Route Addition

To force traffic to a specific LAN device (e.g., 192.168.1.127) through the VPN, add a manual route on the Mac.

### Steps

1. **Open Terminal** on your Mac while connected to the UniFi VPN.
2. **Identify the VPN Gateway IP**:
    - Run `ifconfig` and look for the `utun6` interface to find the assigned VPN IP (e.g., 10.23.100.x).
    - The gateway is typically the UniFi gateway's IP on this subnet (e.g., 10.23.100.1). Confirm this in the UniFi Network app under VPN settings or by checking the VPN client config in the WiFiman app.
3. **Add the Route**:
    - Run the following command, replacing `<VPN_GATEWAY_IP>` with the actual gateway IP:
        
        ```bash
        sudo route -n add -host 192.168.1.127 <VPN_GATEWAY_IP>
        ```
        
    - Example: `sudo route -n add -host 192.168.1.127 10.23.100.1`
    - Enter your admin password when prompted.
4. **Verify the Route**:
    - Run `netstat -rn | grep 192.168.1` to confirm a new entry for 192.168.1.127 via `utun6`.
5. **Test Connectivity**:
    - Ping the device: `ping 192.168.1.127`.
    - If successful, the fix is applied.
6. **Reapply After Disconnect**:
    - This route is temporary and clears on VPN disconnect or reboot. Repeat the command each time you reconnect.

## Long-Term Solutions

- **Disable Split Tunneling**: In UniFi Network app > Settings > VPN > UniFi Identity VPN Server, adjust split tunneling settings or disable it for your user via the WiFiman app to force all traffic through the VPN.
- **Change LAN Subnet**: Avoid conflicts by changing the home LAN subnet (e.g., to 192.168.50.0/24) in UniFi Network app > Settings > Networks > Edit LAN. Update device IPs (e.g., Pi to 192.168.50.127) and renew DHCP leases.
- **Expose Networks**: Ensure `192.168.1.0/24` is listed under "Exposed Networks" in the VPN server settings to push the route automatically.

## Implementation Results

### ✅ Success Criteria Met:
- [x] VPN connects successfully from remote locations
- [x] SSH access works through VPN tunnel after route fix
- [x] MinIO admin interface accessible via VPN
- [x] No ports exposed directly to internet
- [x] VPN performance acceptable for admin tasks

### Testing Evidence:
```bash
# VPN Connection Status
ifconfig utun6                    # Shows 10.23.100.x IP assignment
netstat -rn | grep 192.168.1     # Shows route via utun6 after fix

# Connectivity Tests
ping 192.168.1.127               # ✅ Success after route addition
ssh pi@192.168.1.127             # ✅ SSH access working
curl http://192.168.1.127:9001   # ✅ MinIO admin accessible

# Security Verification
# External network scan (without VPN): All ports closed/filtered ✅
```

## Notes

- Requires admin privileges (`sudo`) on the Mac.
- Firewall rules in UniFi should allow traffic from the VPN subnet (10.23.100.x) to the LAN (192.168.1.x). Check UniFi Network app > Settings > Firewall & Security > Firewall Rules if issues persist.
- Update UniFi OS, Network app, and WiFiman app to the latest versions for bug fixes.
- **Root Cause**: Subnet overlap between local WiFi network (192.168.1.x) and home LAN (192.168.1.x) with split tunneling enabled
- **Key Insight**: Manual host routes override subnet routes and force traffic through VPN tunnel

## Related Issues

- Resolves: Issue #2 - Configure UDM Pro Teleport VPN for Secure Admin Access
- Enables: Issue #1 (CloudFlare Tunnel debugging), Issue #3 (Webhook deployment troubleshooting)

---
*Created: 2025-08-29*  
*Issue: #2 - UDM Pro VPN Configuration*