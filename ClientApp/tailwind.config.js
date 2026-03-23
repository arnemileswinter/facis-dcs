/** @type {import('tailwindcss').Config} */
import tailwindcss from '@tailwindcss/postcss';

export default {
    content: [
        "./index.html",
        "./src/**/*.{vue,js,ts,jsx,tsx}" 
    ],
    theme: {
        extend: {},
    },
    plugins: [],
};
