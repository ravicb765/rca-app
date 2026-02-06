/* Minimal linux/types.h for CI compile-only checks. This intentionally avoids
 * pulling in kernel asm includes (posix/arch) and provides only the small set
 * of types used by our example C files.
 */
#ifndef _LINUX_TYPES_H
#define _LINUX_TYPES_H

typedef unsigned char __u8;
typedef signed char __s8;
typedef unsigned short __u16;
typedef signed short __s16;
typedef unsigned int __u32;
typedef signed int __s32;
typedef unsigned long long __u64;

typedef __u16 __le16;
typedef __u32 __le32;
typedef __u64 __le64;

/* big-endian aliases used in some kernel headers */
typedef __u16 __be16;
typedef __u32 __be32;
typedef __u64 __be64;

/* checksum type used by networking helpers */
typedef unsigned int __wsum;

/* simple aliases for u-types */
typedef __u16 u16;
typedef __u32 u32;

/* aligned 64-bit helper used by some kernel structs */
typedef __u64 __aligned_u64 __attribute__((aligned(8)));

typedef __u64 u64;

typedef long ssize_t;

#endif /* _LINUX_TYPES_H */
