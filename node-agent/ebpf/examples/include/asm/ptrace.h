/* Minimal asm/ptrace.h for compile-only checks. Avoid pulling kernel ptrace/arch details. */
#ifndef _ASM_PTRACE_H
#define _ASM_PTRACE_H

/* Provide a harmless macro for PT_REGS_PARM3 used by examples. */
#ifndef PT_REGS_PARM3
#define PT_REGS_PARM3(x) (0)
#endif

#endif /* _ASM_PTRACE_H */
