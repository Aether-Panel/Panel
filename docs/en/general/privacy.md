# Privacy Policy

## Introduction

Aether Panel is an open source fork of PufferPanel, published under the MIT license. All source code is publicly available on GitHub for review, auditing, and contribution. This privacy policy explains how we handle data on our community site and website.

Since Aether Panel runs on your own server (self-hosted), **we do not have access to your installation or the data it contains**. All information about your servers, users, and configuration stays local on your infrastructure. This policy primarily covers your interaction with our community (Discord, GitHub, website).

We believe in total transparency: as an open source project, anyone can audit the code to verify there is no hidden data collection, telemetry, or backdoors.

## Our Philosophy

Aether Panel (a fork of PufferPanel) was created with a different philosophy:

This philosophy sets us apart from other panels that, as forks or similar projects, choose business models that compromise privacy or software openness.

### Real Open Source, No Tricks

All code is on GitHub under MIT license. No enterprise versions, no hidden premium features, no covert telemetry. What you see is what you get.

### Privacy by Design

The panel runs on your server. We collect nothing. No usage statistics, no telemetry, no trackers. What happens on your server stays on your server.

### Community, Not Customers

You are not a customer because we don't sell anything. You are part of a community. Decisions are made based on what the community needs, not what generates revenue.

### Total Transparency

Being open source (MIT), anyone can audit the code. No closed binaries, no mandatory cloud services, no surprises.


## Data We Collect

**In your panel installation:** We collect absolutely nothing. The panel has no telemetry, no usage reporting, no connections to external servers (except those you configure, like MySQL databases).

**In our community (Discord, GitHub, website):** If you choose to interact with us, we voluntarily collect:

**Important:** If you install the panel, you are solely responsible for your users' data. We have no access, collect no metrics, and cannot see your installation.

### Discord Data (Only if you join)

By joining our Discord server:

This is standard for any Discord server. We do not store this data outside of Discord.

- Username, avatar, and Discord ID (public profile information)
- Messages you send in public channels
- Roles assigned to you (member, contributor, etc.)

### Website Data (Suggestions/Voting)

If you participate in suggestions or voting on the website:

We do not require registration for most interactions. You can participate anonymously.

- Name you provide when submitting a suggestion
- Votes on feature proposals (stored in Firestore)
- Approximate country (determined by IP, for statistical purposes only)

### GitHub Data (Only if you contribute)

If you contribute code to the project:

All this information is public by nature on GitHub.

- GitHub username
- Code and commit messages you submit
- Issues and pull requests you create


## How We Use Your Data

The limited data we collect (community only) is used exclusively for:

**What we do NOT do (and never will):**

All development is funded by voluntary community donations. There is no economic incentive to compromise your privacy.

### Project Improvement

Suggestions and bug reports help us prioritize development of the fork.

### Community Communication

Discord is our main communication channel. Your interactions help us build community.

### Transparent Decision Making

Voting helps us know which features to prioritize. Results are public.


- We do not sell or share personal data with third parties
- We do not implement telemetry in the panel
- We do not collect panel usage statistics
- We have no mandatory cloud services
- We do not monetize user data
- We do not put features behind paywalls — all code is MIT, all free

## Open Source Transparency

As a fork of PufferPanel under the MIT license, the complete source code is available for public audit:

- Public GitHub repository with full change history
- MIT license — you can use, modify, and distribute without restrictions
- Anyone can audit the code to verify no data collection exists
- No closed pre-compiled binaries — everything builds from source
- No mandatory external services — the panel works 100% offline if desired
- Community contributions are welcome and transparent

## Self-Hosted Nature

Aether Panel is designed to be self-hosted. This means:

- You control where and how the panel runs
- All data (users, servers, configurations) stays on your infrastructure
- No mandatory outbound connections — the panel works without internet access
- You are responsible for the security and privacy of data in your installation
- You can audit the panel's network traffic to verify no unwanted communication

## Data Storage

Where and how data is stored depending on context:

### Panel Installation (Your Server)

Local database (SQLite, MySQL, or PostgreSQL per your config):

- User data (username, email, bcrypt-hashed password)
- Panel configuration (config.json on the server)
- Game server files
- Server backups, logs, and databases
- Session and OAuth2 tokens (stored as SHA256 hashes)

### Community (Firebase/Firestore)

Data you voluntarily provide on the website:

- Suggestions and votes (Firestore)
- Testimonials (if you choose to submit one)
- Contact messages (sent to Discord via webhook)

### Data Retention

Retention policy:

- Panel data: you keep it until you decide to delete it
- Discord accounts: as long as you are a server member
- Suggestions and votes: kept as project reference
- Contributed code: permanent (as part of the open source repository)
- You can request deletion of your community data at any time

## Your Rights

We respect your rights over your personal data:

You can request a copy of the data we hold about you in our community.

- **right**: Right of Access
You can request deletion of your data from our community at any time.

- **right**: Right of Deletion
You can export data from your panel installation directly from the database.

- **right**: Right of Portability

### How to Exercise Your Rights

For requests related to data in our community:

For data in your local panel installation, you have full direct control. We cannot access your installation.

- Contact us through Discord
- Specify which data you want to review or delete
- We will process your request within a reasonable timeframe

## Third-Party Services

The project uses minimal external services, each with independent privacy policies:

We do not share data with third parties for advertising, marketing, or any commercial purpose.

Our community server. Subject to Discord's privacy policy. We only access public profile information.

- **name**: Discord
Source code, issues, and pull requests. Everything is public. Subject to GitHub's privacy policy.

- **name**: GitHub
We use Firestore to store suggestions, votes, and testimonials on the website. No sensitive data stored. Subject to Google's policy.

- **name**: Firebase (Google)
For voluntary donations. We do not store or process payment information — everything is handled through PayPal.

- **name**: PayPal

## Minors

The project is not directed at individuals under 13. We do not intentionally collect information from minors.

## Changes to This Policy

If we update this policy:

- We will post an announcement on our Discord
- Update the date on this page
- Substantial changes will be notified in the community

## Contact

For privacy questions or to exercise your rights:

As a non-profit open source project, we respond to the best of our ability. Your privacy matters, and we are committed to protecting it.

- Discord: Join our server and talk to the team
- GitHub: Open an issue in the repository
- The project is maintained by volunteers — no commercial support

## In Summary

Aether Panel is an open source fork of PufferPanel. This means:

Don't trust us, trust the code. Everything is on GitHub for you to verify.

- The code is 100% public and auditable (MIT license)
- The panel runs on YOUR server — we see nothing
- No telemetry, trackers, or data collection
- No paid premium features — everything is free
- We only collect data if you voluntarily interact with our community
- We are a fork — we build on PufferPanel's work and contribute back to the open source community

## Paid Modules

The core Aether Panel is and will always be 100% free and open source (MIT). However, integration modules with third-party software may have an associated cost.

We are currently developing a WHMCS integration module (billing and automation platform). WHMCS is closed-source software that requires a paid license. The WHMCS module for Aether Panel will have a cost because:

The WHMCS module is an optional addon. You don't need to buy it to use the panel. The main panel, all its features, and the panel source code remain 100% free and open source. We will never put panel features behind a paywall.

- WHMCS is proprietary software — requires a paid license for development and testing
- The module requires ongoing maintenance to maintain compatibility
- Cover infrastructure costs for development and testing
- Not for profit — will be sold at the lowest possible price

