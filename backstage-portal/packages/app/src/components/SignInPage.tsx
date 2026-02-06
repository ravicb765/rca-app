import React from 'react';
import {
    SignInPage as BackstageSignInPage,
    SignInProps,
} from '@backstage/core-components';
import {
    githubAuthApiRef,
    googleAuthApiRef,
    microsoftAuthApiRef,
    configApiRef,
    useApi,
} from '@backstage/core-plugin-api';
import { makeStyles, Box, Typography, Container, Paper } from '@material-ui/core';
import { LogoFull } from './Root/Logo';

const useStyles = makeStyles((theme) => ({
    container: {
        height: '100vh',
        width: '100vw',
        display: 'flex',
        flexDirection: 'column',
        alignItems: 'center',
        justifyContent: 'center',
        background: 'radial-gradient(circle at center, #1e293b 0%, #0f172a 100%)',
        position: 'relative',
        overflow: 'hidden',
        '&::before': {
            content: '""',
            position: 'absolute',
            top: '-50%',
            left: '-50%',
            width: '200%',
            height: '200%',
            background: 'conic-gradient(from 0deg at 50% 50%, transparent 0%, rgba(0, 242, 254, 0.05) 50%, transparent 100%)',
            animation: '$rotate 20s linear infinite',
        }
    },
    '@keyframes rotate': {
        from: { transform: 'rotate(0deg)' },
        to: { transform: 'rotate(360deg)' },
    },
    glassPanel: {
        padding: theme.spacing(6),
        background: 'rgba(15, 23, 42, 0.8)',
        backdropFilter: 'blur(16px)',
        borderRadius: '24px',
        border: '1px solid rgba(255, 255, 255, 0.1)',
        boxShadow: '0 25px 50px -12px rgba(0, 0, 0, 0.5)',
        zIndex: 1,
        maxWidth: 480,
        width: '100%',
        textAlign: 'center',
    },
    logoContainer: {
        marginBottom: theme.spacing(4),
        display: 'flex',
        justifyContent: 'center',
    },
    title: {
        color: 'white',
        fontWeight: 800,
        letterSpacing: '-0.5px',
        marginBottom: theme.spacing(1),
        background: 'linear-gradient(90deg, #00F2FE 0%, #4FACFE 100%)',
        WebkitBackgroundClip: 'text',
        WebkitTextFillColor: 'transparent',
    },
    subtitle: {
        color: 'rgba(255, 255, 255, 0.5)',
        marginBottom: theme.spacing(4),
        fontSize: '0.9rem',
    },
    authContainer: {
        '& .MuiButton-root': {
            borderRadius: '12px',
            textTransform: 'none',
            padding: theme.spacing(1.5, 3),
            fontSize: '1rem',
            fontWeight: 600,
            marginBottom: theme.spacing(2),
            width: '100%',
        }
    }
}));

export const SignInPage = (props: SignInProps) => {
    const classes = useStyles();
    const config = useApi(configApiRef);
    const appTitle = config.getOptionalString('app.title') || 'RCA-App Portal';

    return (
        <div className={classes.container}>
            <Box className={classes.glassPanel}>
                <div className={classes.logoContainer}>
                    <LogoFull height={48} />
                </div>
                <Typography variant="h4" className={classes.title}>
                    Welcome Back
                </Typography>
                <Typography variant="body1" className={classes.subtitle}>
                    Secure enterprise entrance for {appTitle}
                </Typography>

                <Box className={classes.authContainer}>
                    <BackstageSignInPage
                        {...props}
                        providers={[
                            'guest',
                            {
                                id: 'github-auth-provider',
                                title: 'Continue with GitHub',
                                message: 'Developer SSO',
                                apiRef: githubAuthApiRef,
                            },
                            {
                                id: 'google-auth-provider',
                                title: 'Continue with Google',
                                message: 'Corporate SSO',
                                apiRef: googleAuthApiRef,
                            },
                            {
                                id: 'microsoft-auth-provider',
                                title: 'Continue with Microsoft',
                                message: 'Enterprise AD',
                                apiRef: microsoftAuthApiRef,
                            },
                        ]}
                    />
                </Box>

                <Box mt={4}>
                    <Typography variant="caption" style={{ color: 'rgba(255,255,255,0.3)' }}>
                        Precision Observability & Root Cause Analysis Platform
                    </Typography>
                </Box>
            </Box>
        </div>
    );
};
