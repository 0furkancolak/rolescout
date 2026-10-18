#!/usr/bin/env python3
"""Generates the synthetic evaluation set (en.json, tr.json).

Every posting is invented from templates with a fixed seed; the labels come from
the template parameters, so they are exact. Run: python3 gen.py
"""
import json
import random

rng = random.Random(42)

ROLES = {
    "Backend Engineer": ["Go", "Java", "Python", "PostgreSQL", "Redis", "Kafka", "Docker", "Kubernetes", "gRPC", "MongoDB"],
    "Full Stack Developer": ["React", "TypeScript", "Node.js", "PostgreSQL", "GraphQL", "Docker", "AWS", "Next.js"],
    "Frontend Engineer": ["React", "TypeScript", "Vue", "Next.js", "GraphQL", "JavaScript", "Angular"],
    "Data Engineer": ["Python", "Spark", "Airflow", "Snowflake", "Kafka", "AWS", "BigQuery", "PostgreSQL"],
    "DevOps Engineer": ["Kubernetes", "Terraform", "AWS", "Docker", "Ansible", "Jenkins", "Linux", "Azure"],
    "Software Engineer": ["Python", "Java", "Kotlin", "C#", "PostgreSQL", "Docker", "Redis"],
    "Mobile Developer": ["Kotlin", "Swift", "React", "TypeScript", "GraphQL"],
    "Machine Learning Engineer": ["Python", "PyTorch", "TensorFlow", "Spark", "AWS", "Docker"],
}
COMPANIES = ["Acme Labs", "Northwind", "Globex", "Initech", "Umbrella Systems", "Hooli", "Stark Software", "Wayne Digital", "Aperture", "Vandelay",
             "Cyberdyne", "Soylent Tech", "Massive Dynamic", "Gringotts Pay", "Tyrell Cloud"]
TR_COMPANIES = ["Mavi Yazılım", "Anadolu Teknoloji", "Boğaziçi Dijital", "Kızılay Bilişim", "Ege Yazılım", "Marmara Sistemleri", "Toros Teknoloji",
                "Bosna Software", "Kapadokya Dijital", "Karadeniz Bilişim", "Efes Cloud", "Truva Teknoloji", "Pamukkale Yazılım", "Çınar Bilişim", "Gediz Software"]
# city -> (country code, English country, Turkish country)
CITIES = {
    "Berlin": ("DE", "Germany", "Almanya"), "Dublin": ("IE", "Ireland", "İrlanda"), "Amsterdam": ("NL", "the Netherlands", "Hollanda"),
    "London": ("GB", "the United Kingdom", "Birleşik Krallık"), "Austin": ("US", "the United States", "ABD"), "Warsaw": ("PL", "Poland", "Polonya"),
    "İstanbul": ("TR", "Turkey", "Türkiye"), "Ankara": ("TR", "Turkey", "Türkiye"), "İzmir": ("TR", "Turkey", "Türkiye"),
}
EN_CITIES = ["Berlin", "Dublin", "Amsterdam", "London", "Austin", "Warsaw"]
TR_CITIES = ["İstanbul", "Ankara", "İzmir"]

SENIORITY_EN = {"intern": "Intern", "junior": "Junior", "mid": "Mid-level", "senior": "Senior", "lead": "Tech Lead", "unknown": ""}
SENIORITY_TR = {"intern": "Stajyer", "junior": "Junior", "mid": "Mid-level", "senior": "Kıdemli", "lead": "Tech Lead", "unknown": ""}


def pick_techs(role):
    pool = ROLES[role]
    return rng.sample(pool, rng.randint(3, 5))


def money_en():
    kind = rng.choice(["usd", "eur", "gbp", "eur_month"])
    if kind == "usd":
        lo = rng.randrange(80, 150, 5) * 1000
        hi = lo + rng.randrange(10, 40, 5) * 1000
        return f"Salary range: ${lo:,} - ${hi:,} per year.", (lo, hi, "USD", "year")
    if kind == "eur":
        lo = rng.randrange(55, 100, 5)
        hi = lo + rng.randrange(10, 30, 5)
        return f"Compensation: €{lo}k - €{hi}k a year.", (lo * 1000, hi * 1000, "EUR", "year")
    if kind == "gbp":
        lo = rng.randrange(40, 80, 5) * 1000
        hi = lo + rng.randrange(5, 20, 5) * 1000
        return f"The salary is £{lo:,} to £{hi:,} per annum.", (lo, hi, "GBP", "year")
    lo = rng.randrange(40, 70, 5) * 100
    hi = lo + rng.randrange(5, 15, 5) * 100
    return f"Pay: €{lo:,} - €{hi:,} per month.", (lo, hi, "EUR", "month")


def money_tr():
    kind = rng.choice(["tl", "usd"])
    if kind == "tl":
        lo = rng.randrange(60, 160, 10) * 1000
        hi = lo + rng.randrange(10, 40, 10) * 1000
        return f"Maaş: {lo:,} - {hi:,} TL aylık.".replace(",", "."), (lo, hi, "TRY", "month")
    lo = rng.randrange(20, 45, 5) * 100
    hi = lo + rng.randrange(5, 15, 5) * 100
    return f"Ücret: {lo:,} - {hi:,} USD aylık.".replace(",", "."), (lo, hi, "USD", "month")


def build_en(i):
    role = rng.choice(list(ROLES))
    level = rng.choices(["intern", "junior", "mid", "senior", "lead", "unknown"], [1, 3, 3, 8, 3, 8])[0]
    prefix = SENIORITY_EN[level]
    title = f"{prefix} {role}".strip()
    if level == "intern":
        title = f"{role} Intern"
    company = COMPANIES[i % len(COMPANIES)]
    techs = pick_techs(role)
    arrangement = rng.choice(["remote", "hybrid", "onsite", "unknown"])
    city = rng.choice(EN_CITIES)
    code, country, _ = CITIES[city]
    parts = [f"{company} is hiring a {title.lower() if level == 'unknown' else title} to join our product team."]
    parts.append(f"You will work with {', '.join(techs[:-1])} and {techs[-1]} to build and run customer-facing services.")
    if level == "unknown":
        parts.append(f"We expect {rng.randint(2, 6)}+ years of professional experience.")
    country_code = ""
    if arrangement == "remote":
        parts.append(rng.choice(["This is a fully remote role.", "Work from anywhere in the EU, fully remote."]))
    elif arrangement == "hybrid":
        parts.append(f"Hybrid: {rng.randint(2, 3)} days a week in our {city} office.")
        country_code = code
    elif arrangement == "onsite":
        parts.append(f"The role is on-site in {city}, {country}.")
        country_code = code
    else:
        parts.append("Our team collaborates closely and values clear writing.")
    parts.append("Benefits include a learning budget, equity and 28 days of leave.")
    salary = None
    if rng.random() < 0.45:
        text, salary = money_en()
        parts.append(text)
    employment = "unknown"
    return {
        "id": f"en-{i + 1:02d}", "lang": "en", "title": title, "company": company, "description": "\n\n".join(parts),
        "expect": {
            "seniority": level, "work_arrangement": arrangement, "country_code": country_code,
            "technologies": sorted({t.lower() for t in techs}),
            "salary_min": salary[0] if salary else None, "salary_max": salary[1] if salary else None,
            "salary_currency": salary[2] if salary else "", "salary_period": salary[3] if salary else "unknown",
        },
    }


def build_tr(i):
    role = rng.choice(list(ROLES))
    level = rng.choices(["intern", "junior", "mid", "senior", "lead", "unknown"], [2, 3, 3, 8, 2, 8])[0]
    prefix = SENIORITY_TR[level]
    title = f"{prefix} {role}".strip()
    if level == "intern":
        title = f"Yazılım Stajyeri ({role})"
    company = TR_COMPANIES[i % len(TR_COMPANIES)]
    techs = pick_techs(role)
    arrangement = rng.choice(["remote", "hybrid", "onsite", "unknown"])
    city = rng.choice(TR_CITIES)
    code = CITIES[city][0]
    parts = [f"{company} olarak ürün ekibimize katılacak bir {title} arıyoruz."]
    parts.append(f"{', '.join(techs[:-1])} ve {techs[-1]} ile müşterilerimize dönük servisler geliştireceksiniz.")
    if level == "unknown":
        parts.append(f"En az {rng.randint(2, 6)} yıl profesyonel deneyim beklenir.")
    country_code = ""
    if arrangement == "remote":
        parts.append(rng.choice(["Tamamen uzaktan çalışma modeli.", "Türkiye'nin her yerinden uzaktan çalışabilirsiniz."]))
    elif arrangement == "hybrid":
        parts.append(f"Hibrit çalışma modeli: haftada {rng.randint(2, 3)} gün {city} ofisinde.")
        country_code = code
    elif arrangement == "onsite":
        parts.append(f"Pozisyon {city} ofisinde, ofisten çalışılır.")
        country_code = code
    else:
        parts.append("Ekibimiz açık iletişime ve düzenli kod incelemesine önem verir.")
    parts.append("Yan haklar arasında özel sağlık sigortası, eğitim bütçesi ve yemek kartı bulunur.")
    salary = None
    if rng.random() < 0.45:
        text, salary = money_tr()
        parts.append(text)
    return {
        "id": f"tr-{i + 1:02d}", "lang": "tr", "title": title, "company": company, "description": "\n\n".join(parts),
        "expect": {
            "seniority": level, "work_arrangement": arrangement, "country_code": country_code,
            "technologies": sorted({t.lower() for t in techs}),
            "salary_min": salary[0] if salary else None, "salary_max": salary[1] if salary else None,
            "salary_currency": salary[2] if salary else "", "salary_period": salary[3] if salary else "unknown",
        },
    }


if __name__ == "__main__":
    for name, build in (("en", build_en), ("tr", build_tr)):
        cases = [build(i) for i in range(30)]
        with open(f"{name}.json", "w", encoding="utf-8") as f:
            json.dump(cases, f, ensure_ascii=False, indent=1)
            f.write("\n")
