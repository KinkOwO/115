__int64 sub_1444EDFD0()
{
  __int64 result; // rax
  unsigned int v1; // edi
  int v2; // esi
  __int64 v3; // rcx
  __int64 v4; // rax
  __int64 v5; // rbx
  __int64 v6; // rax
  __int64 v7; // rax
  __int64 v8; // rax
  __int64 v9; // rax
  unsigned int v10; // [rsp+20h] [rbp-78h] BYREF
  _BYTE v11[4]; // [rsp+28h] [rbp-70h] BYREF
  _BYTE v12[4]; // [rsp+2Ch] [rbp-6Ch] BYREF
  __int64 v13; // [rsp+30h] [rbp-68h]
  __int128 v14; // [rsp+38h] [rbp-60h]
  __int64 v15; // [rsp+48h] [rbp-50h]
  __int128 v16; // [rsp+50h] [rbp-48h]
  __int64 v17; // [rsp+60h] [rbp-38h]
  __int128 v18; // [rsp+68h] [rbp-30h]
  __int64 v19; // [rsp+78h] [rbp-20h]
  unsigned __int8 v20; // [rsp+B0h] [rbp+18h] BYREF
  char v21; // [rsp+B8h] [rbp+20h] BYREF

  v13 = -2;
  v20 = 0;
  result = sub_146EA09F0(&v20, 1);
  if ( v20 )
  {
    v1 = 1;
    do
    {
      v2 = v1;
      v21 = 10;
      sub_146EA09F0(&v21, 1);
      if ( v21 == 10 )
      {
        if ( !qword_14E638F28 )
        {
          qword_14E638F28 = sub_1444E7F30();
          (**(void (__fastcall ***)(__int64))qword_14E638F28)(qword_14E638F28);
        }
        v14 = 0u;
        v15 = 0;
        sub_146EA0BA0(v11);
        v3 = qword_14E638F28;
        if ( !qword_14E638F28 )
        {
          qword_14E638F28 = sub_1444E7F30();
          (**(void (__fastcall ***)(__int64))qword_14E638F28)(qword_14E638F28);
          v3 = qword_14E638F28;
        }
        sub_1444EFA00(v3, 1, v1);
        if ( !qword_14E638F28 )
        {
          qword_14E638F28 = sub_1444E7F30();
          (**(void (__fastcall ***)(__int64))qword_14E638F28)(qword_14E638F28);
        }
        v16 = 0u;
        v17 = 0;
        sub_146EA0BA0(&v10);
        v4 = sub_143C61150();
        sub_1447EF3B0(v4, v10, v1);
        if ( !qword_14E638F28 )
        {
          qword_14E638F28 = sub_1444E7F30();
          (**(void (__fastcall ***)(__int64))qword_14E638F28)(qword_14E638F28);
        }
        v18 = 0u;
        v19 = 0;
        v5 = 4;
        do
        {
          sub_146EA0BA0(v12);
          --v5;
        }
        while ( v5 );
        v6 = sub_140764510();
        sub_1444EFA00(v6, 4, v1);
        v7 = sub_140764510();
        sub_1444EFA00(v7, 6, v1);
        v8 = sub_140764510();
        sub_1444EFA00(v8, 7, v1);
        v9 = sub_140764510();
        sub_1444EFA00(v9, 10, v1);
      }
      ++v1;
      result = v20;
    }
    while ( v2 < v20 );
  }
  return result;
}
