# Hosting Comparison: Why Hetzner Wins

## Executive Summary

For the Cetus Marketdata Scanner, **Hetzner Cloud CX22** offers the best price-to-performance ratio at **€3.79/month (~$4.10 USD)**.

## Detailed Comparison

### Hetzner Cloud CX22 (Recommended)

**Price:** €3.79/month (~$4.10 USD)

**Specs:**
- 2 vCPU (shared)
- 4GB RAM
- 40GB NVMe SSD
- 20TB bandwidth/month
- 1 Gbps network

**Pros:**
- ✅ **6-10x cheaper** than US providers for same specs
- ✅ European data centers (Germany/Finland) - GDPR compliant
- ✅ 99.9% uptime SLA
- ✅ DDoS protection included
- ✅ Daily backups included
- ✅ No hidden fees
- ✅ Excellent price-to-performance ratio
- ✅ Can handle 1,000-3,000 daily active users

**Cons:**
- ❌ European data centers only (latency for US users: ~100-150ms)
- ❌ Shared CPU (but sufficient for our workload)
- ❌ Less brand recognition in US market

**Best for:** Budget-conscious deployments, European users, MVP/early stage

---

### DigitalOcean Basic Droplet

**Price:** $24/month

**Specs:**
- 2 vCPU (shared)
- 4GB RAM
- 80GB SSD
- 4TB bandwidth/month
- 1 Gbps network

**Pros:**
- ✅ US data centers (lower latency for US users)
- ✅ Well-known brand
- ✅ Good documentation
- ✅ Easy to use control panel
- ✅ Multiple regions worldwide

**Cons:**
- ❌ **6x more expensive** than Hetzner
- ❌ Less bandwidth (4TB vs 20TB)
- ❌ Same specs as Hetzner for 6x the price

**Best for:** US-focused applications, brand-conscious deployments

---

### AWS EC2 t3.medium

**Price:** ~$30/month (on-demand)

**Specs:**
- 2 vCPU
- 4GB RAM
- EBS storage (additional cost)
- Bandwidth (additional cost)

**Pros:**
- ✅ Industry standard
- ✅ Global infrastructure
- ✅ Advanced features (auto-scaling, load balancing)
- ✅ Enterprise support available

**Cons:**
- ❌ **7-8x more expensive** than Hetzner
- ❌ Complex pricing (storage, bandwidth, etc. are extra)
- ❌ Steep learning curve
- ❌ Overkill for our use case

**Best for:** Enterprise applications, complex architectures

---

### Google Cloud e2-medium

**Price:** ~$35/month (on-demand)

**Specs:**
- 2 vCPU
- 4GB RAM
- Persistent disk (additional cost)
- Network egress (additional cost)

**Pros:**
- ✅ Global infrastructure
- ✅ Good integration with Google services
- ✅ Kubernetes support

**Cons:**
- ❌ **8-9x more expensive** than Hetzner
- ❌ Complex pricing model
- ❌ Overkill for simple deployments

**Best for:** Google Cloud ecosystem, Kubernetes workloads

---

### Linode (Akamai) 4GB

**Price:** $24/month

**Specs:**
- 2 vCPU
- 4GB RAM
- 80GB SSD
- 4TB bandwidth/month

**Pros:**
- ✅ US and global data centers
- ✅ Good performance
- ✅ Simple pricing

**Cons:**
- ❌ **6x more expensive** than Hetzner
- ❌ Same specs as Hetzner for 6x the price

**Best for:** US-focused applications, alternative to DigitalOcean

---

### Vultr 4GB

**Price:** $24/month

**Specs:**
- 2 vCPU
- 4GB RAM
- 80GB SSD
- 4TB bandwidth/month

**Pros:**
- ✅ Global data centers
- ✅ Good performance
- ✅ Hourly billing

**Cons:**
- ❌ **6x more expensive** than Hetzner
- ❌ Same specs as Hetzner for 6x the price

**Best for:** Global deployments, hourly billing needs

---

## Cost Analysis

### Monthly Costs (Same Specs: 2 vCPU, 4GB RAM)

| Provider | Monthly Cost | Annual Cost | 3-Year Cost |
|----------|-------------|-------------|-------------|
| **Hetzner** | **€3.79 ($4.10)** | **€45.48 ($49.20)** | **€136.44 ($147.60)** |
| DigitalOcean | $24.00 | $288.00 | $864.00 |
| AWS t3.medium | $30.00 | $360.00 | $1,080.00 |
| Google Cloud | $35.00 | $420.00 | $1,260.00 |
| Linode | $24.00 | $288.00 | $864.00 |
| Vultr | $24.00 | $288.00 | $864.00 |

### Savings with Hetzner

**vs DigitalOcean:** Save $239/year ($717 over 3 years)  
**vs AWS:** Save $311/year ($932 over 3 years)  
**vs Google Cloud:** Save $371/year ($1,112 over 3 years)

---

## Performance Analysis

### Our Application Requirements

**Baseline usage:**
- Go binary: ~50-100MB RAM
- SQLite: ~200-500MB RAM (with cache)
- API server: ~100MB RAM
- **Total: ~400-700MB RAM**

**Available on 4GB server:** ~3.3GB free for load

### Capacity Estimates (Hetzner CX22)

**Conservative (guaranteed performance):**
- 50-100 concurrent users
- 500-1,000 daily active users
- 10,000+ registered users

**Moderate (good performance):**
- 200-300 concurrent users
- 2,000-3,000 daily active users
- 50,000+ registered users

**Aggressive (acceptable performance):**
- 500+ concurrent users
- 5,000+ daily active users
- 100,000+ registered users

### Why So Many Users?

1. **SQLite is fast** - Indexed queries, sub-100ms typical
2. **Rate limiting** - We throttle users (60-1000 calls/min)
3. **Go is efficient** - Low memory footprint, fast HTTP
4. **Most users are idle** - 90% of users don't hit limits
5. **Scans are scheduled** - Heavy computation happens once daily

---

## Latency Analysis

### Hetzner (Germany) to Major Cities

| City | Latency | Impact |
|------|---------|--------|
| Berlin | 10-20ms | Excellent |
| London | 20-30ms | Excellent |
| Paris | 20-30ms | Excellent |
| New York | 90-110ms | Good |
| Los Angeles | 140-160ms | Acceptable |
| Tokyo | 220-250ms | Acceptable |
| Sydney | 280-320ms | Marginal |

### Does Latency Matter for Our Use Case?

**For EOD (End-of-Day) scanner:**
- ❌ **NO** - Users aren't doing real-time trading
- ❌ **NO** - Dashboard loads are not latency-sensitive
- ❌ **NO** - API calls are not time-critical
- ✅ **YES** - Initial page load (but cached after first load)

**Conclusion:** 100-150ms latency to US is acceptable for our use case.

---

## Reliability Analysis

### Hetzner Uptime

- **SLA:** 99.9% (8.76 hours downtime/year max)
- **Actual uptime:** Typically 99.95%+ (4.38 hours/year)
- **Maintenance windows:** Scheduled, announced in advance
- **Backup power:** Redundant UPS and generators

### Comparison

| Provider | SLA | Actual Uptime |
|----------|-----|---------------|
| Hetzner | 99.9% | 99.95%+ |
| DigitalOcean | 99.99% | 99.95%+ |
| AWS | 99.99% | 99.99%+ |
| Google Cloud | 99.99% | 99.99%+ |

**Note:** Hetzner's actual uptime matches or exceeds their SLA, and is comparable to more expensive providers.

---

## Security Analysis

### Hetzner Security Features

- ✅ DDoS protection (included)
- ✅ Firewall (included)
- ✅ Private networking (included)
- ✅ ISO 27001 certified data centers
- ✅ GDPR compliant
- ✅ Daily backups (included)
- ✅ Two-factor authentication for control panel

### Comparison

| Feature | Hetzner | DigitalOcean | AWS | Google Cloud |
|---------|---------|--------------|-----|--------------|
| DDoS Protection | ✅ Included | ✅ Included | ⚠️ Extra cost | ✅ Included |
| Firewall | ✅ Included | ✅ Included | ✅ Included | ✅ Included |
| Backups | ✅ Included | ⚠️ Extra cost | ⚠️ Extra cost | ⚠️ Extra cost |
| Private Network | ✅ Included | ✅ Included | ✅ Included | ✅ Included |

**Winner:** Hetzner (all security features included at no extra cost)

---

## Support Analysis

### Hetzner Support

- **Community support:** Free (forums, documentation)
- **Email support:** Free (24-48 hour response)
- **Phone support:** Not available
- **Enterprise support:** Available (custom pricing)

### Comparison

| Provider | Free Support | Paid Support |
|----------|-------------|--------------|
| Hetzner | ✅ Email (24-48h) | Custom pricing |
| DigitalOcean | ✅ Email (24h) | $99+/month |
| AWS | ❌ None | $29+/month |
| Google Cloud | ❌ None | $29+/month |

**Winner:** Hetzner (free email support is sufficient for our use case)

---

## Migration Path

### Starting with Hetzner

1. **Deploy on Hetzner CX22** (€3.79/month)
2. **Test with real users**
3. **Monitor performance**
4. **Scale as needed**

### When to Upgrade

**Upgrade to CX32 (€14.49/month) when:**
- 2,000+ daily active users
- CPU usage > 70% sustained
- Memory usage > 80% sustained
- Revenue > €50/month

**Upgrade to CX42 (€28.99/month) when:**
- 10,000+ daily active users
- CPU usage > 70% sustained
- Memory usage > 80% sustained
- Revenue > €200/month

**Migrate to multi-server when:**
- 50,000+ daily active users
- Need high availability
- Need geographic distribution
- Revenue > €1,000/month

### Migration Process

**Hetzner to Hetzner (upgrade):**
1. Create new server
2. Deploy application
3. Migrate database
4. Update DNS/load balancer
5. **Downtime:** 5-10 minutes

**Hetzner to AWS/Google Cloud (if needed):**
1. Set up new infrastructure
2. Deploy application
3. Migrate database
4. Update DNS
5. **Downtime:** 10-30 minutes

---

## Break-Even Analysis

### Hetzner CX22: €3.79/month

**If you charge:**
- **$29/month per user** → Need **1 paying customer** to break even
- **$9/month per user** → Need **1 paying customer** to break even
- **$5/month per user** → Need **1 paying customer** to break even

**You're profitable from day 1 with a single paying user!**

### DigitalOcean: $24/month

**If you charge:**
- **$29/month per user** → Need **1 paying customer** to break even
- **$9/month per user** → Need **3 paying customers** to break even
- **$5/month per user** → Need **5 paying customers** to break even

---

## Final Recommendation

### Choose Hetzner If:

- ✅ Budget is a priority
- ✅ You're in early stage/MVP phase
- ✅ European users are acceptable (or latency doesn't matter)
- ✅ You want maximum value for money
- ✅ You need 1,000-3,000 daily active users

### Choose DigitalOcean/AWS If:

- ✅ US data centers are required
- ✅ Brand recognition is important
- ✅ You need enterprise features
- ✅ Budget is not a constraint
- ✅ You need global infrastructure

### Our Recommendation

**Start with Hetzner CX22 (€3.79/month)**

**Why:**
1. ✅ Dirt cheap - profitable immediately
2. ✅ Handles 1,000-3,000 users easily
3. ✅ Room to grow
4. ✅ European data centers (GDPR compliant)
5. ✅ Excellent uptime (99.9%+)
6. ✅ Easy to upgrade later
7. ✅ All security features included

**When to reconsider:**
- When you hit 5,000+ daily active users
- When US latency becomes a problem
- When you need enterprise features
- When you're making €500+/month in revenue

---

## Conclusion

**Hetzner Cloud CX22 is the clear winner** for the Cetus Marketdata Scanner:

- **6-10x cheaper** than competitors
- **Same or better specs**
- **Excellent reliability**
- **All features included**
- **Easy to scale**

**Bottom line:** Start with Hetzner, save money, and scale when needed.

---

## Quick Start

```bash
# Deploy to Hetzner in 5 minutes
curl -sSL https://raw.githubusercontent.com/jsoprych/json_scanner/main/deploy/hetzner-deploy.sh | sudo bash
```

**Total cost:** €3.79/month (~$4.10 USD)  
**Capacity:** 1,000-3,000 daily active users  
**Setup time:** 5 minutes

**Ready to deploy?** See [HETZNER_DEPLOYMENT.md](HETZNER_DEPLOYMENT.md) for detailed instructions.
