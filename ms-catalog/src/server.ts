import { setupTracing } from './tracer'

setupTracing('ms-catalog');

import express from 'express';

const app = express();
const PORT = 3333;

app.use(express.json());
app.get('/get_product', async (req, res) => {

    const sleep = async (milliseconds: number) => {
        return new Promise((resolve) => setTimeout(resolve, milliseconds));
    };
    await sleep(3000);
    return res.status(200).send({ 'id': '123', 'name': 'Example Product', 'price': 49.99 });
});

app.listen(PORT, () => {
    console.log(`Listening on http://localhost:${PORT}`);
});