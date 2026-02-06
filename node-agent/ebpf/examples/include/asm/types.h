/* Minimal stub of <asm/types.h> to satisfy linux/types.h in CI compile-only
 * This is intentionally tiny and only provides the basic integer typedefs used by
 * our examples when kernel asm headers are unavailable on hosted runners.
 */
#ifndef _ASM_TYPES_H
#define _ASM_TYPES_H

typedef unsigned short __u16;
typedef unsigned int __u32;
typedef unsigned long long __u64;

typedef __u16 __le16;
typedef __u32 __le32;
typedef __u64 __le64;

#endif /* _ASM_TYPES_H */
