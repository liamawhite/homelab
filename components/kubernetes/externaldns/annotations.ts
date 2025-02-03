export const hostname = (domain: string) => ({
    'external-dns.alpha.kubernetes.io/hostname': domain,
})
