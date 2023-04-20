import { setupTracing } from './tracer'

setupTracing('ms-authorization');

import express from 'express';

const app = express();
const PORT = 8081;

app.use(express.json());
app.get('/authorize', async (req, res) => {

    const sleep = async (milliseconds: number) => {
        return new Promise((resolve) => setTimeout(resolve, milliseconds));
    };
    await sleep(2000);
    return res.status(200).send({ status: 'authorized' });
});

app.listen(PORT, () => {
    console.log(`Listening on http://localhost:${PORT}`);
});