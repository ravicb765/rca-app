import React from 'react';
import { makeStyles } from '@material-ui/core';
import { Link } from '@backstage/core-components';
import { LogoFull, LogoIcon } from './Logo';

const useStyles = makeStyles(theme => ({
    sidebarLogo: {
        display: 'flex',
        width: '100%',
        padding: theme.spacing(2),
        justifyContent: 'center',
    },
}));

export const SidebarLogo = () => {
    const classes = useStyles();
    return (
        <div className={classes.sidebarLogo}>
            <Link to="/" underline="none">
                <LogoFull height={40} />
            </Link>
        </div>
    );
};
