/* Minimal compatibility typedefs used by the eBPF examples during CI compile-only runs.
 * This is intentionally small: it avoids pulling in full kernel arch headers that
 * contain inline assembly which clang may reject for compile-only checks on hosted runners.
 */
#ifndef __COMPAT_TYPES_H
#define __COMPAT_TYPES_H

typedef unsigned long long __u64;
typedef long long __s64;
typedef unsigned int __u32;

typedef __u64 u64;

/* Provide a harmless PT_REGS_PARM3 for compile-only checks (avoid pulling
 * in full kernel ptrace/arch headers on hosted runners). Macro returns 0.
 */
#ifndef PT_REGS_PARM3
#define PT_REGS_PARM3(x) (0)
#endif

typedef long ssize_t;

#endif /* __COMPAT_TYPES_H */
