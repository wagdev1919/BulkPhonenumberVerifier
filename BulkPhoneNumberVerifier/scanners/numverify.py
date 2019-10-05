#!/usr/bin/env python3
# -*- coding:utf-8 -*-
#
# @name   : PhoneInfoga - Phone numbers OSINT tool
# @url    : https://github.com/sundowndev
# @author : Raphael Cerveaux (sundowndev)

from bs4 import BeautifulSoup
import hashlib
import json
from lib.output import *
from lib.request import send

'''
Scanning phone number
return: 
 0: Success
-1: phone number format error. Error: Please specify a valid phone number. Example: +6464806649
-2: connection error to https://numverify.com/
'''
def scan(number, proxy):

    #test('Running Numverify.com scan...')

    try:
        requestSecret = ''
        res = send('GET', 'https://numverify.com/', {}, proxy)
        soup = BeautifulSoup(res.text, "html5lib")
    except Exception as e:
        error('Numverify.com is not available')
        #error(e)
        return -2, {'valid':False}

    for tag in soup.find_all("input", type="hidden"):
        if tag['name'] == "scl_request_secret":
            requestSecret = tag['value']
            break

    apiKey = hashlib.md5((number + requestSecret).encode('utf-8')).hexdigest()

    headers = {
        'Host': 'numverify.com',
        'User-Agent': 'Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:64.0) Gecko/20100101 Firefox/64.0',
        'Accept': 'application/json, text/javascript, */*; q=0.01',
        'Accept-Language': 'fr,fr-FR;q=0.8,en-US;q=0.5,en;q=0.3',
        'Accept-Encoding': 'gzip, deflate, br',
        'Referer': 'https://numverify.com/',
        'X-Requested-With': 'XMLHttpRequest',
        'DNT': '1',
        'Connection': 'keep-alive',
        'Pragma': 'no-cache',
        'Cache-Control': 'no-cache'
    }

    try:
        res = send("GET", "https://numverify.com/php_helper_scripts/phone_api.php?secret_key={}&number={}".format(apiKey, number), headers, proxy)
        data = json.loads(res.content.decode('utf-8'))
    except Exception as e:
        #error('Numverify.com is not available')
        return -2, {'valid':False}

    if res.content == "Unauthorized" or res.status_code != 200:
        #error(("An error occured while calling the API (bad request or wrong api key)."))
        return -2, {'valid':False}

    if 'error' in data:
        #error('Numverify.com is not available: ' + data['error'])
        return -2, {'valid':False}

    return 0, data
