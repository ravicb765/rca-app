import React from 'react';
import { makeStyles } from '@material-ui/core';

const useStyles = makeStyles({
    svg: {
        width: 'auto',
        height: (props: any) => props.height || 32,
    },
});

export const LogoIcon = (props: { height?: number }) => {
    const classes = useStyles(props);
    return (
        <svg className={classes.svg} viewBox="0 0 256 256" fill="none" xmlns="http://www.w3.org/2000/svg">
            <defs>
                <linearGradient id="rca-grad" x1="0" y1="0" x2="256" y2="256" gradientUnits="userSpaceOnUse">
                    <stop stopColor="#00F2FE" />
                    <stop offset="1" stopColor="#4FACFE" />
                </linearGradient>
            </defs>
            <path d="M128 32 L224 88 V200 L128 256 L32 200 V88 L128 32Z" fill="url(#rca-grad)" fillOpacity="0.1" stroke="url(#rca-grad)" strokeWidth="2" />
            <path d="M80 176 V80 H144 C162.2 80 176 93.8 176 112 C176 130.2 162.2 144 144 144 H80 M144 144 L176 176"
                stroke="white" strokeWidth="12" strokeLinecap="round" strokeLinejoin="round" />
            <circle cx="176" cy="176" r="10" fill="#FF3D71" />
        </svg>
    );
};

export const LogoFull = (props: { height?: number }) => {
    const classes = useStyles(props);
    return (
        <div style={{ display: 'flex', alignItems: 'center', gap: '12px' }}>
            <LogoIcon height={props.height || 32} />
            <span style={{
                color: 'white',
                fontSize: '20px',
                fontWeight: 700,
                letterSpacing: '1px',
                fontFamily: 'Roboto, sans-serif'
            }}>
                RCA-App
            </span>
        </div>
    );
};
